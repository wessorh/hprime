/*
 * warcfp_fast — GPU-accelerated WARC fingerprinting.
 * Strategy: precompute Lanczos weights ON THE GPU, not CPU.
 * This eliminates the sin/cos bottleneck that made the CPU version win.
 *
 * Build:  nvcc -O3 -o warcfp_fast warcfp_fast.cu -lm
 */

/* Prevent glibc >= 2.40 from declaring sinpi/cospi (guarded by __USE_GNU
   which comes from _GNU_SOURCE that nvcc defines by default). These conflict
   with CUDA 12.x's declarations. CUDA headers provide all math we need. */
#undef _GNU_SOURCE
#undef __USE_GNU
#include <cuda_runtime.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <ctype.h>

#define FP_SIZE      16
#define BITS         14
#define SCALE_F      ((float)(1 << BITS))
#define ROUND_F      ((float)(1 << (BITS - 1)))
#define PI_F         3.141592653589793f
#define MAX_TAPS     2048
#define MAX_PAGE     8388608
#define BATCH_SIZE   256
#define MAX_URL      512

typedef struct {
    uint8_t  *data;
    int32_t   len;
    char      url[MAX_URL];
} Page;

typedef struct {
    Page       pages[BATCH_SIZE];
    int        count;
    int32_t    page_lens[BATCH_SIZE];  /* for GPU */
    /* GPU buffers */
    uint8_t   *d_inputs;
    int32_t   *d_offsets;
    int32_t   *d_lengths;
    int16_t   *d_weights;    /* computed ON GPU */
    int32_t   *d_L;
    uint8_t   *d_outputs;
} Batch;

/* ─── Lanczos-3 kernel (GPU) ────────────────────────────────────────── */
__device__ inline float l3f(float x) {
    if (x > 3.0f) return 0.0f;
    if (x == 0.0f) return 1.0f;
    float b = x * PI_F;
    float c = b / 3.0f;
    return __sinf(b) * __sinf(c) / (b * c);
}

/*
 * GPU kernel: compute Lanczos weights AND fingerprints for a batch.
 * Grid: N blocks (one per page)
 * Block: 16 threads (one per fingerprint byte)
 *
 * Each thread:
 *   1. Computes Lanczos weights for its output byte from the page length
 *   2. Does dot product with the page content
 *   3. Writes one fingerprint byte
 */
__global__ void fp_kernel_full(
    const uint8_t *inputs,      /* packed page contents */
    const int32_t *offsets,     /* start of each page in inputs */
    const int32_t *lengths,     /* page lengths */
    int16_t       *weights,     /* output: computed weights [page][byte][tap] */
    uint8_t       *outputs,     /* output: fingerprints */
    int            batch_count
) {
    int page_idx = blockIdx.x;
    if (page_idx >= batch_count) return;

    int byte_idx = threadIdx.x;
    if (byte_idx >= FP_SIZE) return;

    int page_len = lengths[page_idx];
    if (page_len < 1) page_len = 1;
    int offset  = offsets[page_idx];

    /* ─── Step 1: compute Lanczos weights for this (page, byte) ─── */
    float flen = (float)page_len;
    float ctr  = ((float)byte_idx + 0.5f) * flen / (float)FP_SIZE;
    float sup  = 3.0f * flen / (float)FP_SIZE;

    int L = (int)ceilf(ctr - sup);
    int R = (int)floorf(ctr + sup);
    if (L < 0) L = 0;
    if (R >= page_len) R = page_len - 1;
    int n = R - L + 1;
    if (n > MAX_TAPS) {
        int extra = (n - MAX_TAPS) / 2;
        L += extra;
        n = MAX_TAPS;
        R = L + n - 1;
    }

    /* Compute weights and sum simultaneously */
    float sum = 0.0f;
    float ws[MAX_TAPS];
    for (int j = L; j <= R; j++) {
        float x = (ctr - (float)j) * (float)FP_SIZE / flen;
        float w = l3f(fabsf(x));
        ws[j - L] = w;
        sum += w;
    }

    /* Normalize and convert to fixed-point */
    float sw = SCALE_F / sum;
    int16_t *my_weights = weights + (page_idx * FP_SIZE + byte_idx) * MAX_TAPS;
    for (int j = 0; j < n; j++) {
        float wt = ws[j] * sw;
        my_weights[j] = (int16_t)(wt < 0.0f ? wt - 0.5f : wt + 0.5f);
    }
    for (int j = n; j < MAX_TAPS; j++) my_weights[j] = 0;

    __syncthreads();

    /* ─── Step 2: dot product ─── */
    const uint8_t *in = inputs + offset + L;
    int32_t dot_sum = 0;
    for (int j = 0; j < n; j++) {
        dot_sum += (int32_t)in[j] * (int32_t)my_weights[j];
    }

    __syncthreads();

    /* ─── Step 3: finalize ─── */
    int v = (int)(((float)dot_sum + ROUND_F) / SCALE_F);
    if (v < 0) v = 0; if (v > 255) v = 255;
    outputs[page_idx * FP_SIZE + byte_idx] = (uint8_t)v;
}

/* ─── Batch management ──────────────────────────────────────────────── */
static void batch_init(Batch *b) {
    b->count = 0;
    for (int i = 0; i < BATCH_SIZE; i++) {
        b->pages[i].data = NULL;
        b->pages[i].len = 0;
    }
    b->d_inputs  = NULL;
    b->d_offsets = NULL;
    b->d_lengths = NULL;
    b->d_weights = NULL;
    b->d_L       = NULL;
    b->d_outputs = NULL;
}

static void batch_free_gpu(Batch *b) {
    if (b->d_inputs)  cudaFree(b->d_inputs);
    if (b->d_offsets) cudaFree(b->d_offsets);
    if (b->d_lengths) cudaFree(b->d_lengths);
    if (b->d_weights) cudaFree(b->d_weights);
    if (b->d_L)       cudaFree(b->d_L);
    if (b->d_outputs) cudaFree(b->d_outputs);
    b->d_inputs  = NULL;
    b->d_offsets = NULL;
    b->d_lengths = NULL;
    b->d_weights = NULL;
    b->d_L       = NULL;
    b->d_outputs = NULL;
}

static void batch_process(Batch *b) {
    if (b->count == 0) return;
    int N = b->count;

    /* Pack inputs and collect lengths */
    size_t total = 0;
    for (int i = 0; i < N; i++) {
        total += (size_t)b->pages[i].len;
        b->page_lens[i] = b->pages[i].len;
    }

    /* Allocate GPU memory */
    cudaMalloc(&b->d_inputs,  total);
    cudaMalloc(&b->d_offsets, (N + 1) * sizeof(int32_t));
    cudaMalloc(&b->d_lengths, N * sizeof(int32_t));
    cudaMalloc(&b->d_weights, N * FP_SIZE * MAX_TAPS * sizeof(int16_t));
    cudaMalloc(&b->d_outputs, N * FP_SIZE * sizeof(uint8_t));

    /* Pack and transfer */
    uint8_t  *h_in  = (uint8_t *)malloc(total);
    int32_t  *h_off = (int32_t *)malloc((N + 1) * sizeof(int32_t));
    size_t off = 0;
    for (int i = 0; i < N; i++) {
        h_off[i] = (int32_t)off;
        memcpy(h_in + off, b->pages[i].data, (size_t)b->pages[i].len);
        off += (size_t)b->pages[i].len;
    }
    h_off[N] = (int32_t)total;

    cudaMemcpy(b->d_inputs,  h_in,  total, cudaMemcpyHostToDevice);
    cudaMemcpy(b->d_offsets, h_off, (N + 1) * sizeof(int32_t), cudaMemcpyHostToDevice);
    cudaMemcpy(b->d_lengths, b->page_lens, N * sizeof(int32_t), cudaMemcpyHostToDevice);
    free(h_in); free(h_off);

    /* Launch GPU kernel: computes weights AND fingerprints */
    fp_kernel_full<<<N, FP_SIZE>>>(
        b->d_inputs, b->d_offsets, b->d_lengths,
        b->d_weights, b->d_outputs, N
    );
    cudaDeviceSynchronize();

    /* Read back and emit */
    uint8_t *h_out = (uint8_t *)malloc(N * FP_SIZE);
    cudaMemcpy(h_out, b->d_outputs, N * FP_SIZE, cudaMemcpyDeviceToHost);

    for (int i = 0; i < N; i++) {
        uint8_t *fp = h_out + i * FP_SIZE;
        for (int j = 0; j < FP_SIZE; j++) printf("%02x", fp[j]);
        printf("  %s\n", b->pages[i].url);
    }
    fflush(stdout);
    free(h_out);

    for (int i = 0; i < N; i++) { free(b->pages[i].data); b->pages[i].data = NULL; }
    b->count = 0;
    batch_free_gpu(b);
}

/* ─── WARC Parser (same as before) ──────────────────────────────────── */
static const uint8_t *skip_http_headers(const uint8_t *buf, int len) {
    for (int i = 0; i < len - 3; i++)
        if (buf[i]=='\r'&&buf[i+1]=='\n'&&buf[i+2]=='\r'&&buf[i+3]=='\n')
            return buf + i + 4;
    return NULL;
}

static int extract_header(const char *line, const char *key, char *value, int vsz) {
    int kl = (int)strlen(key);
    if (strncasecmp(line, key, kl) != 0) return 0;
    const char *v = line + kl;
    while (*v == ' ' || *v == ':') v++;
    const char *e = v;
    while (*e && *e != '\r' && *e != '\n') e++;
    int vl = (int)(e - v);
    if (vl >= vsz) vl = vsz - 1;
    memcpy(value, v, (size_t)vl); value[vl] = '\0';
    return 1;
}

static int parse_warc(const uint8_t *buf, int len, char *url, int usz,
                       const uint8_t **body, int *blen) {
    if (len < 20 || memcmp(buf, "WARC/", 5) != 0) return 0;
    int he = -1;
    for (int i = 0; i < len - 3; i++)
        if (buf[i]=='\r'&&buf[i+1]=='\n'&&buf[i+2]=='\r'&&buf[i+3]=='\n')
            { he = i; break; }
    if (he < 0) return 0;

    char wtype[64]={0}, clstr[32]={0}; url[0]='\0';
    int pos = 0;
    while (pos < he) {
        int le = pos;
        while (le < he && !(buf[le]=='\r'&&buf[le+1]=='\n')) le++;
        int ll = le - pos;
        if (ll > 0 && ll < 4095) {
            char line[4096]; memcpy(line, buf+pos, (size_t)ll); line[ll]=0;
            extract_header(line, "WARC-Type", wtype, sizeof(wtype));
            extract_header(line, "WARC-Target-URI", url, usz);
            extract_header(line, "Content-Length", clstr, sizeof(clstr));
        }
        pos = le + 2;
    }
    if (strcasecmp(wtype, "response") != 0 || url[0]=='\0') {
        int cl = clstr[0] ? atoi(clstr) : 0;
        int total = he + 4 + cl;
        *body = NULL; *blen = 0;  /* prevent caller from reusing stale body */
        return total > 0 ? total : len;
    }
    int cl = clstr[0] ? atoi(clstr) : 0;
    if (cl <= 0 || cl > MAX_PAGE) return he + 4 + cl;
    int bs = he + 4;
    if (bs + cl > len) return 0;
    const uint8_t *hb = skip_http_headers(buf + bs, cl);
    if (hb) { *blen = cl - (int)(hb - (buf + bs)); *body = hb; }
    else    { *blen = cl; *body = buf + bs; }
    return (*blen > 0) ? (bs + cl) : (bs + cl);
}

static void process_warc(const char *fn, Batch *b) {
    FILE *f = stdin;
    if (fn && strcmp(fn, "--stdin") != 0) {
        f = fopen(fn, "rb"); if (!f) return;
    }
    fseek(f, 0, SEEK_END); long fl = ftell(f); fseek(f, 0, SEEK_SET);
    if (fl <= 0) { if (f!=stdin) fclose(f); return; }
    uint8_t *d = (uint8_t *)malloc((size_t)fl);
    if (!d || fread(d,1,(size_t)fl,f)!=(size_t)fl) { free(d); if(f!=stdin)fclose(f); return; }
    if (f!=stdin) fclose(f);

    int off = 0;
    while (off < (int)fl) {
        char url[MAX_URL]; const uint8_t *body; int blen;
        int c = parse_warc(d+off, (int)(fl-off), url, sizeof(url), &body, &blen);
        if (c <= 0) { off++; while (off<(int)fl-5 && memcmp(d+off,"WARC/",5)!=0) off++; continue; }
        if (body && blen > 0 && url[0]) {
            Page *p = &b->pages[b->count];
            p->data = (uint8_t *)malloc((size_t)blen);
            memcpy(p->data, body, (size_t)blen);
            p->len = blen;
            strncpy(p->url, url, sizeof(p->url)-1);
            p->url[sizeof(p->url)-1] = '\0';
            if (++b->count >= BATCH_SIZE) batch_process(b);
        }
        off += c;
    }
    free(d);
    if (b->count > 0) batch_process(b);
}

int main(int argc, char **argv) {
    int nd; cudaGetDeviceCount(&nd);
    if (nd == 0) { fprintf(stderr,"No CUDA device\n"); return 1; }
    cudaDeviceProp p; cudaGetDeviceProperties(&p, 0);
    fprintf(stderr, "GPU: %s\n", p.name);

    if (argc < 2) {
        fprintf(stderr, "Usage: %s <file.warc> [file2.warc ...]\n", argv[0]);
        return 1;
    }
    Batch *b = (Batch *)malloc(sizeof(Batch));
    batch_init(b);
    for (int i = 1; i < argc; i++) process_warc(argv[i], b);
    free(b);
    return 0;
}
