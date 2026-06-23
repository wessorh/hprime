/*
 * warcfp — WARC file parser with CUDA-batched 1D Lanczos fingerprinting.
 *
 * Streams WARC files, extracts URL + page content from "response" records,
 * batches pages to GPU for parallel fingerprint computation, emits:
 *   <32-hex-char-fingerprint>  <URL>
 *
 * Build:
 *   nvcc -O3 -o warcfp warcfp.cu -lm
 *
 * Usage:
 *   ./warcfp <file.warc> [file2.warc ...]
 *   ./warcfp --stdin           (read WARC from stdin)
 */

#include <cuda_runtime.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <time.h>
#include <ctype.h>

/* ─── Constants ─────────────────────────────────────────────────────── */
#define FP_SIZE      16
#define BITS         14
#define SCALE        (1 << BITS)
#define ROUND        (1 << (BITS - 1))
#define PI           3.141592653589793
#define MAX_TAPS     2048
#define MAX_PAGE     8388608   /* 8 MB max page size */
#define BATCH_SIZE   128        /* pages per GPU batch */
#define MAX_URL      512

/* ─── Types ─────────────────────────────────────────────────────────── */

/* Precomputed Lanczos kernel for one page (CPU builds, GPU reads) */
typedef struct {
    int32_t L[FP_SIZE];        /* input offset for each output byte */
    int32_t n;                 /* number of taps (same for all outputs after truncation) */
    int16_t weights[FP_SIZE * MAX_TAPS];  /* packed: [byte0_w0..w2047, byte1_w0..w2047, ...] */
    int32_t page_len;          /* original page length */
} PageKernel;

/* One page ready for GPU processing */
typedef struct {
    uint8_t  *data;            /* page content (CPU memory) */
    int32_t   len;             /* page length in bytes */
    char      url[MAX_URL];    /* WARC-Target-URI */
} Page;

/* Batch of pages for one GPU launch */
typedef struct {
    Page       pages[BATCH_SIZE];
    int        count;
    PageKernel kernels[BATCH_SIZE];      /* CPU-side precomputed kernels */
    /* GPU buffers */
    uint8_t   *d_inputs;                 /* packed input pages */
    int32_t   *d_offsets;               /* start offset of each page in d_inputs */
    int16_t   *d_weights;               /* packed weights */
    int32_t   *d_L;                     /* per-output-byte offsets */
    int32_t   *d_n;                     /* tap count per page */
    uint8_t   *d_outputs;               /* BATCH_SIZE × FP_SIZE output */
} Batch;

/* ─── Lanczos kernel ────────────────────────────────────────────────── */
__host__ __device__ static inline double l3(double x) {
    if (x > 3.0) return 0.0;
    if (x == 0.0) return 1.0;
    double b = x * PI, c = b / 3.0;
    return sin(b) * sin(c) / (b * c);
}

__host__ __device__ static inline uint8_t cl8(int x) {
    return (uint8_t)(x < 0 ? 0 : (x > 255 ? 255 : x));
}

/* ─── CPU: precompute Lanczos kernels for a page ────────────────────── */
static void build_kernel(PageKernel *k, int page_len) {
    if (page_len < 1) page_len = 1;
    double flen = (double)page_len;
    int actual_taps = 0;

    for (int i = 0; i < FP_SIZE; i++) {
        double ctr = (i + 0.5) * flen / FP_SIZE;
        double sup = 3.0 * flen / FP_SIZE;
        int L = (int)ceil(ctr - sup);
        int R = (int)floor(ctr + sup);
        if (L < 0) L = 0;
        if (R >= page_len) R = page_len - 1;
        int n = R - L + 1;
        if (n > MAX_TAPS) {
            int extra = (n - MAX_TAPS) / 2;
            L += extra;
            n = MAX_TAPS;
        }
        R = L + n - 1;
        k->L[i] = L;
        if (n > actual_taps) actual_taps = n;

        double sum = 0.0, ws[MAX_TAPS];
        for (int j = L; j <= R; j++) {
            double x = (ctr - j) * FP_SIZE / flen;
            ws[j - L] = l3(fabs(x));
            sum += ws[j - L];
        }
        double sw = (double)SCALE / sum;
        int16_t *w = &k->weights[i * MAX_TAPS];
        for (int j = 0; j < n; j++) {
            double wt = ws[j] * sw;
            w[j] = (int16_t)(wt < 0 ? wt - 0.5 : wt + 0.5);
        }
        /* zero-pad remaining taps */
        for (int j = n; j < MAX_TAPS; j++) w[j] = 0;
    }
    k->n = actual_taps;
    k->page_len = page_len;
}

/* ─── CUDA kernel: compute fingerprints for a batch ─────────────────── */
__global__ void fp_kernel(
    const uint8_t *inputs,      /* packed input pages */
    const int32_t *offsets,     /* start offset of each page */
    const int16_t *weights,     /* packed weights [page][byte][tap] */
    const int32_t *L,           /* per-output-byte offsets [page][16] */
    const int32_t *n,           /* tap count per page */
    uint8_t *outputs,           /* BATCH_SIZE × FP_SIZE output */
    int batch_count
) {
    int page_idx = blockIdx.x;
    if (page_idx >= batch_count) return;

    int byte_idx = threadIdx.x;  /* 0..15, one per fingerprint byte */
    if (byte_idx >= FP_SIZE) return;

    int offset = offsets[page_idx];
    int tap_count = n[page_idx];
    int L_val = L[page_idx * FP_SIZE + byte_idx];

    const uint8_t *in = inputs + offset + L_val;
    const int16_t *w = weights + (page_idx * FP_SIZE + byte_idx) * MAX_TAPS;

    /* Serial dot product — 2048 elements, warp is fine with this */
    int32_t sum = 0;
    for (int j = 0; j < tap_count; j++) {
        sum += (int32_t)in[j] * (int32_t)w[j];
    }

    __syncthreads();

    if (byte_idx < FP_SIZE) {
        int v = (sum + ROUND) >> BITS;
        outputs[page_idx * FP_SIZE + byte_idx] = cl8(v);
    }
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
    b->d_weights = NULL;
    b->d_L       = NULL;
    b->d_n       = NULL;
    b->d_outputs = NULL;
}

static void batch_free_gpu(Batch *b) {
    if (b->d_inputs)  cudaFree(b->d_inputs);
    if (b->d_offsets) cudaFree(b->d_offsets);
    if (b->d_weights) cudaFree(b->d_weights);
    if (b->d_L)       cudaFree(b->d_L);
    if (b->d_n)       cudaFree(b->d_n);
    if (b->d_outputs) cudaFree(b->d_outputs);
    b->d_inputs  = NULL;
    b->d_offsets = NULL;
    b->d_weights = NULL;
    b->d_L       = NULL;
    b->d_n       = NULL;
    b->d_outputs = NULL;
}

/* Transfer batch to GPU, launch kernel, read back results */
static void batch_process(Batch *b) {
    if (b->count == 0) return;

    int N = b->count;
    size_t total_input = 0;
    for (int i = 0; i < N; i++)
        total_input += (size_t)b->pages[i].len;
    cudaMalloc(&b->d_inputs,  total_input);
    cudaMalloc(&b->d_offsets, (N + 1) * sizeof(int32_t));
    cudaMalloc(&b->d_weights, N * FP_SIZE * MAX_TAPS * sizeof(int16_t));
    cudaMalloc(&b->d_L,       N * FP_SIZE * sizeof(int32_t));
    cudaMalloc(&b->d_n,       N * sizeof(int32_t));
    cudaMalloc(&b->d_outputs, N * FP_SIZE * sizeof(uint8_t));

    /* Pack and transfer inputs */
    uint8_t  *host_inputs  = (uint8_t *)malloc(total_input);
    int32_t  *host_offsets = (int32_t *)malloc((N + 1) * sizeof(int32_t));
    int16_t  *host_weights = (int16_t *)malloc(N * FP_SIZE * MAX_TAPS * sizeof(int16_t));
    int32_t  *host_L       = (int32_t *)malloc(N * FP_SIZE * sizeof(int32_t));
    int32_t  *host_n       = (int32_t *)malloc(N * sizeof(int32_t));

    size_t off = 0;
    for (int i = 0; i < N; i++) {
        host_offsets[i] = (int32_t)off;
        memcpy(host_inputs + off, b->pages[i].data, (size_t)b->pages[i].len);
        off += (size_t)b->pages[i].len;

        memcpy(host_weights + i * FP_SIZE * MAX_TAPS,
               b->kernels[i].weights, FP_SIZE * MAX_TAPS * sizeof(int16_t));
        memcpy(host_L + i * FP_SIZE, b->kernels[i].L, FP_SIZE * sizeof(int32_t));
        host_n[i] = b->kernels[i].n;
    }
    host_offsets[N] = (int32_t)total_input;

    cudaMemcpy(b->d_inputs,  host_inputs,  total_input, cudaMemcpyHostToDevice);
    cudaMemcpy(b->d_offsets, host_offsets, (N + 1) * sizeof(int32_t), cudaMemcpyHostToDevice);
    cudaMemcpy(b->d_weights, host_weights, N * FP_SIZE * MAX_TAPS * sizeof(int16_t), cudaMemcpyHostToDevice);
    cudaMemcpy(b->d_L,       host_L,       N * FP_SIZE * sizeof(int32_t), cudaMemcpyHostToDevice);
    cudaMemcpy(b->d_n,       host_n,       N * sizeof(int32_t), cudaMemcpyHostToDevice);

    free(host_inputs);
    free(host_offsets);
    free(host_weights);
    free(host_L);
    free(host_n);

    /* Launch kernel: N blocks, 16 threads per block */
    fp_kernel<<<N, FP_SIZE>>>(
        b->d_inputs, b->d_offsets, b->d_weights, b->d_L, b->d_n,
        b->d_outputs, N
    );
    cudaDeviceSynchronize();

    /* Read back and emit */
    uint8_t *host_out = (uint8_t *)malloc(N * FP_SIZE);
    cudaMemcpy(host_out, b->d_outputs, N * FP_SIZE * sizeof(uint8_t), cudaMemcpyDeviceToHost);

    for (int i = 0; i < N; i++) {
        uint8_t *fp = host_out + i * FP_SIZE;
        for (int j = 0; j < FP_SIZE; j++) printf("%02x", fp[j]);
        printf("  %s\n", b->pages[i].url);
    }
    fflush(stdout);

    free(host_out);

    /* Cleanup CPU pages */
    for (int i = 0; i < N; i++) {
        free(b->pages[i].data);
        b->pages[i].data = NULL;
    }
    b->count = 0;

    batch_free_gpu(b);
}

/* ─── WARC Parser ───────────────────────────────────────────────────── */

/* Skip HTTP response headers to get to the body.
   Returns pointer to start of body within buf, or NULL. */
static const uint8_t *skip_http_headers(const uint8_t *buf, int len) {
    for (int i = 0; i < len - 3; i++) {
        if (buf[i] == '\r' && buf[i+1] == '\n' &&
            buf[i+2] == '\r' && buf[i+3] == '\n') {
            return buf + i + 4;
        }
    }
    return NULL;
}

/* Extract header value from a WARC header line.
   Line looks like: "Key: value\r\n" or "Key: value" */
static int extract_header(const char *line, const char *key,
                          char *value, int value_sz) {
    int klen = (int)strlen(key);
    if (strncasecmp(line, key, klen) != 0) return 0;
    const char *val = line + klen;
    while (*val == ' ' || *val == ':') val++;
    const char *end = val;
    while (*end != '\r' && *end != '\n' && *end != '\0') end++;
    int vlen = (int)(end - val);
    if (vlen >= value_sz) vlen = value_sz - 1;
    memcpy(value, val, (size_t)vlen);
    value[vlen] = '\0';
    return 1;
}

/* Parse one WARC record from buf[len]. Returns bytes consumed, or 0. */
static int parse_warc_record(const uint8_t *buf, int len,
                             char *url_out, int url_sz,
                             const uint8_t **body_out, int *body_len) {
    if (len < 20) return 0;

    /* Check for WARC version line */
    if (memcmp(buf, "WARC/", 5) != 0) return 0;

    /* Find end of WARC headers (blank line: \r\n\r\n) */
    int header_end = -1;
    for (int i = 0; i < len - 3; i++) {
        if (buf[i] == '\r' && buf[i+1] == '\n' &&
            buf[i+2] == '\r' && buf[i+3] == '\n') {
            header_end = i;
            break;
        }
    }
    if (header_end < 0) return 0;

    /* Parse headers from the header block */
    char warc_type[64] = {0};
    char content_length_str[32] = {0};
    url_out[0] = '\0';

    int pos = 0;
    while (pos < header_end) {
        /* Find end of this line */
        int line_end = pos;
        while (line_end < header_end &&
               !(buf[line_end] == '\r' && buf[line_end+1] == '\n'))
            line_end++;
        int line_len = line_end - pos;
        if (line_len <= 0) { pos = line_end + 2; continue; }

        char line[4096];
        int copy = line_len < 4095 ? line_len : 4095;
        memcpy(line, buf + pos, (size_t)copy);
        line[copy] = '\0';

        extract_header(line, "WARC-Type", warc_type, sizeof(warc_type));
        extract_header(line, "WARC-Target-URI", url_out, url_sz);
        extract_header(line, "Content-Length", content_length_str, sizeof(content_length_str));

        pos = line_end + 2; /* skip \r\n */
    }

    /* Only process "response" records */
    if (strcasecmp(warc_type, "response") != 0) {
        /* Consume the record even if we don't process it */
        int clen = content_length_str[0] ? atoi(content_length_str) : 0;
        int body_start = header_end + 4; /* skip \r\n\r\n */
        int total = body_start + clen;
        if (total > len) total = len;
        /* Align to next WARC record boundary (\r\n\r\n) */
        while (total < len - 3 &&
               !(buf[total] == '\r' && buf[total+1] == '\n' &&
                 buf[total+2] == 'W' && buf[total+3] == 'A'))
            total++;
        *body_out = NULL; *body_len = 0;  /* prevent stale reuse */
        return total > 0 ? total : len;
    }

    if (url_out[0] == '\0') {
        /* No URL found — skip */
        *body_out = NULL; *body_len = 0;
        return header_end + 4;
    }

    int clen = content_length_str[0] ? atoi(content_length_str) : 0;
    if (clen <= 0 || clen > MAX_PAGE) {
        return header_end + 4 + clen;
    }

    int body_start = header_end + 4; /* past \r\n\r\n */
    if (body_start + clen > len) {
        return 0; /* incomplete record — need more data */
    }

    const uint8_t *http_body = skip_http_headers(buf + body_start, clen);
    if (http_body) {
        int hdr_skip = (int)(http_body - (buf + body_start));
        *body_len = clen - hdr_skip;
        *body_out = http_body;
    } else {
        /* No HTTP headers found — use entire content as body */
        *body_len = clen;
        *body_out = buf + body_start;
    }

    if (*body_len <= 0) {
        return body_start + clen;
    }

    return body_start + clen;
}

/* ─── Process one WARC file ─────────────────────────────────────────── */
static void process_warc(const char *filename, Batch *batch) {
    FILE *f = stdin;
    if (filename && strcmp(filename, "--stdin") != 0) {
        f = fopen(filename, "rb");
        if (!f) {
            fprintf(stderr, "error: cannot open %s\n", filename);
            return;
        }
    }

    /* Read entire file into memory (WARC files are typically compressed,
       but we handle uncompressed .warc here) */
    fseek(f, 0, SEEK_END);
    long flen_orig = ftell(f);
    fseek(f, 0, SEEK_SET);

    size_t flen = (size_t)(flen_orig > 0 ? flen_orig : 0);
    if (flen == 0) {
        if (f != stdin) fclose(f);
        return;
    }

    uint8_t *data = (uint8_t *)malloc(flen);
    if (!data) {
        if (f != stdin) fclose(f);
        return;
    }
    size_t nr = fread(data, 1, flen, f);
    if (f != stdin) fclose(f);
    if (nr != flen) { free(data); return; }

    int offset = 0;
    while (offset < (int)flen) {
        char url[MAX_URL];
        const uint8_t *body;
        int body_len;

        int consumed = parse_warc_record(data + offset, (int)(flen - offset),
                                          url, sizeof(url), &body, &body_len);
        if (consumed <= 0) {
            /* Try to find next WARC record */
            int next = offset + 1;
            while (next < (int)flen - 5 &&
                   memcmp(data + next, "WARC/", 5) != 0) next++;
            if (next >= (int)flen - 5) break;
            offset = next;
            continue;
        }

        if (body && body_len > 0 && url[0] != '\0') {            /* Copy page data */
            Page *p = &batch->pages[batch->count];
            p->data = (uint8_t *)malloc((size_t)body_len);
            memcpy(p->data, body, (size_t)body_len);
            p->len = body_len;
            strncpy(p->url, url, sizeof(p->url) - 1);
            p->url[sizeof(p->url) - 1] = '\0';

            /* Precompute Lanczos kernel */
            build_kernel(&batch->kernels[batch->count], body_len);

            batch->count++;

            if (batch->count >= BATCH_SIZE) {
                batch_process(batch);
            }
        }

        offset += consumed;
    }

    free(data);

    /* Process any remaining pages */
    if (batch->count > 0) {        batch_process(batch);
    }}

/* ─── Main ───────────────────────────────────────────────────────────── */
int main(int argc, char **argv) {
    /* Check for CUDA device */
    int dev_count;
    if (cudaGetDeviceCount(&dev_count) != cudaSuccess || dev_count == 0) {
        fprintf(stderr, "No CUDA device found. Aborting.\n");
        return 1;
    }
    cudaDeviceProp prop;
    cudaGetDeviceProperties(&prop, 0);
    fprintf(stderr, "Using CUDA device: %s\n", prop.name);

    Batch *batch = (Batch *)malloc(sizeof(Batch));
    if (!batch) { fprintf(stderr, "Out of memory\n"); return 1; }
    batch_init(batch);

    if (argc < 2) {
        fprintf(stderr, "Usage: %s <file.warc> [file2.warc ...]\n", argv[0]);
        fprintf(stderr, "       %s --stdin\n", argv[0]);
        free(batch);
        return 1;
    }

    for (int i = 1; i < argc; i++) {
        process_warc(argv[i], batch);
    }

    free(batch);
    return 0;
}
