/*
 * fptool — 1D Lanczos file fingerprinting with AVX2, parallel directory walk.
 *
 * Usage:
 *   fptool <file>              fingerprint one file
 *   fptool <directory>         fingerprint all files recursively (parallel)
 *   fptool -j N <directory>    use N threads (default: CPU count)
 *
 * Output: one line per file:  <32-hex-chars>  <filename>
 *
 * Build:
 *   gcc -O3 -mavx2 -o fptool main.c -lm -lpthread
 */

#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <math.h>
#include <time.h>
#include <dirent.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <pthread.h>
#include <unistd.h>
#include <immintrin.h>

#define FP_SIZE      16
#define BITS         14
#define SCALE        (1 << BITS)
#define ROUND        (1 << (BITS - 1))
#define PI           3.141592653589793
#define MAX_TAPS     2048
#define QUEUE_SIZE   256

/* ─── AVX2 detection ─────────────────────────────────────────────────── */
static int has_avx2(void) {
    unsigned eax, ebx, ecx, edx;
    __asm__ __volatile__("cpuid" : "=b"(ebx), "=c"(ecx) : "a"(7), "c"(0) : "edx");
    return (ebx >> 5) & 1;
}

/* ─── Lanczos-3 kernel ───────────────────────────────────────────────── */
static inline double l3(double x) {
    if (x > 3.0) return 0.0;
    if (x == 0.0) return 1.0;
    double b = x * PI, c = b / 3.0;
    return sin(b) * sin(c) / (b * c);
}

static inline uint8_t cl8(int x) {
    return (uint8_t)(x < 0 ? 0 : (x > 255 ? 255 : x));
}

/* ─── 1D Lanczos kernel (precomputed weights) ────────────────────────── */
typedef struct {
    int32_t offset;
    int32_t ntaps;
    int16_t weights[MAX_TAPS];
} Kernel1D;

static Kernel1D *build_kernels(size_t input_len) {
    Kernel1D *k = (Kernel1D *)malloc(FP_SIZE * sizeof(Kernel1D));
    if (!k) return NULL;
    double ilen = (double)input_len;
    for (int i = 0; i < FP_SIZE; i++) {
        double ctr = (i + 0.5) * ilen / FP_SIZE;
        double sup = 3.0 * ilen / FP_SIZE;
        int L = (int)ceil(ctr - sup);
        int R = (int)floor(ctr + sup);
        if (L < 0) L = 0;
        if (R >= (int)input_len) R = (int)input_len - 1;
        int n = R - L + 1;
        if (n > MAX_TAPS) {
            int extra = (n - MAX_TAPS) / 2;
            L += extra;
            n = MAX_TAPS;
        }
        R = L + n - 1;
        k[i].offset = L;
        k[i].ntaps = n;

        double sum = 0.0, ws[MAX_TAPS];
        for (int j = L; j <= R; j++) {
            double x = (ctr - j) * FP_SIZE / ilen;
            ws[j - L] = l3(fabs(x));
            sum += ws[j - L];
        }
        double sw = (double)SCALE / sum;
        for (int j = 0; j < n; j++) {
            double w = ws[j] * sw;
            k[i].weights[j] = (int16_t)(w < 0 ? w - 0.5 : w + 0.5);
        }
    }
    return k;
}

/* ─── AVX2 dot product ───────────────────────────────────────────────── */
static int32_t dot_avx2(const uint8_t *input, const Kernel1D *k) {
    int L = k->offset, n = k->ntaps;
    const uint8_t *p = input + L;
    const int16_t *w = k->weights;

    __m256i acc = _mm256_setzero_si256();
    const __m256i rnd = _mm256_set1_epi32(ROUND);

    int j = 0;
    for (; j + 8 <= n; j += 8) {
        __m128i pu8 = _mm_loadl_epi64((const __m128i *)&p[j]);
        __m256i pi32 = _mm256_cvtepu8_epi32(pu8);
        __m128i wi16 = _mm_loadu_si128((__m128i *)&w[j]);
        __m256i wi32 = _mm256_cvtepi16_epi32(wi16);
        acc = _mm256_add_epi32(acc, _mm256_mullo_epi32(pi32, wi32));
    }

    __m128i lo = _mm256_castsi256_si128(acc);
    __m128i hi = _mm256_extracti128_si256(acc, 1);
    __m128i s = _mm_add_epi32(lo, hi);
    s = _mm_hadd_epi32(s, s);
    s = _mm_hadd_epi32(s, s);
    int32_t sum = _mm_cvtsi128_si32(s);

    for (; j < n; j++) sum += (int32_t)p[j] * (int32_t)w[j];

    return sum;
}

/* ─── Scalar fallback ────────────────────────────────────────────────── */
static int32_t dot_scalar(const uint8_t *input, const Kernel1D *k) {
    int L = k->offset, n = k->ntaps;
    const uint8_t *p = input + L;
    const int16_t *w = k->weights;
    int32_t sum = 0;
    for (int j = 0; j < n; j++) sum += (int32_t)p[j] * (int32_t)w[j];
    return sum;
}

/* ─── Generate fingerprint ───────────────────────────────────────────── */
static void fingerprint_buffer(const uint8_t *data, size_t len,
                                uint8_t fp[FP_SIZE], int use_avx2) {
    Kernel1D *k = build_kernels(len);
    if (!k) return;
    for (int i = 0; i < FP_SIZE; i++) {
        int32_t s = use_avx2 ? dot_avx2(data, &k[i])
                             : dot_scalar(data, &k[i]);
        int v = (s + ROUND) >> BITS;
        fp[i] = cl8(v);
    }
    free(k);
}

/* ─── Read entire file ───────────────────────────────────────────────── */
static uint8_t *read_file(const char *path, size_t *out_len) {
    FILE *f = fopen(path, "rb");
    if (!f) return NULL;
    fseek(f, 0, SEEK_END);
    size_t len = (size_t)ftell(f);
    fseek(f, 0, SEEK_SET);
    uint8_t *buf = (uint8_t *)malloc(len ? len : 1);
    if (!buf) { fclose(f); return NULL; }
    if (len > 0) {
        size_t nr = fread(buf, 1, len, f);
        if (nr != len) { free(buf); fclose(f); return NULL; }
    }
    fclose(f);
    *out_len = len;
    return buf;
}

/* ─── Print fingerprint ──────────────────────────────────────────────── */
static void print_fp(const uint8_t fp[FP_SIZE], const char *path) {
    for (int i = 0; i < FP_SIZE; i++) printf("%02x", fp[i]);
    printf("  %s\n", path);
}

/* ─── Fingerprint a single file ──────────────────────────────────────── */
static int fp_file(const char *path, int use_avx2) {
    size_t len;
    uint8_t *data = read_file(path, &len);
    if (!data) {
        fprintf(stderr, "error: cannot read %s\n", path);
        return -1;
    }
    uint8_t fp[FP_SIZE];
    fingerprint_buffer(data, len, fp, use_avx2);
    print_fp(fp, path);
    free(data);
    return 0;
}

/* ─── Parallel directory walk ────────────────────────────────────────── */

typedef struct { char path[4096]; } WorkItem;

static WorkItem work_queue[QUEUE_SIZE];
static int queue_head = 0, queue_tail = 0;
static int queue_done = 0;
static pthread_mutex_t queue_mutex = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t  queue_cond  = PTHREAD_COND_INITIALIZER;
static pthread_mutex_t print_mutex = PTHREAD_MUTEX_INITIALIZER;
static int use_avx2_global = 0;

static void enqueue(const char *path) {
    pthread_mutex_lock(&queue_mutex);
    while ((queue_tail + 1) % QUEUE_SIZE == queue_head) {
        /* queue full — worker should be draining it; brief wait */
        pthread_mutex_unlock(&queue_mutex);
        usleep(1000);
        pthread_mutex_lock(&queue_mutex);
    }
    strncpy(work_queue[queue_tail].path, path, sizeof(work_queue[0].path) - 1);
    work_queue[queue_tail].path[sizeof(work_queue[0].path) - 1] = '\0';
    queue_tail = (queue_tail + 1) % QUEUE_SIZE;
    pthread_cond_signal(&queue_cond);
    pthread_mutex_unlock(&queue_mutex);
}

static int dequeue(char *path, size_t path_sz) {
    pthread_mutex_lock(&queue_mutex);
    while (queue_head == queue_tail && !queue_done)
        pthread_cond_wait(&queue_cond, &queue_mutex);
    if (queue_head == queue_tail && queue_done) {
        pthread_mutex_unlock(&queue_mutex);
        return 0;
    }
    strncpy(path, work_queue[queue_head].path, path_sz - 1);
    path[path_sz - 1] = '\0';
    queue_head = (queue_head + 1) % QUEUE_SIZE;
    pthread_mutex_unlock(&queue_mutex);
    return 1;
}

static void walk_dir(const char *dirpath) {
    DIR *d = opendir(dirpath);
    if (!d) return;
    struct dirent *ent;
    while ((ent = readdir(d)) != NULL) {
        if (ent->d_name[0] == '.') continue;
        char full[4096];
        snprintf(full, sizeof(full), "%s/%s", dirpath, ent->d_name);
        struct stat st;
        if (stat(full, &st) != 0) continue;
        if (S_ISDIR(st.st_mode)) {
            walk_dir(full);
        } else if (S_ISREG(st.st_mode)) {
            enqueue(full);
        }
    }
    closedir(d);
}

static void *worker(void *arg) {
    (void)arg;
    char path[4096];
    while (dequeue(path, sizeof(path))) {
        size_t len;
        uint8_t *data = read_file(path, &len);
        if (!data) continue;
        uint8_t fp[FP_SIZE];
        fingerprint_buffer(data, len, fp, use_avx2_global);
        pthread_mutex_lock(&print_mutex);
        print_fp(fp, path);
        fflush(stdout);
        pthread_mutex_unlock(&print_mutex);
        free(data);
    }
    return NULL;
}

/* ─── Main ───────────────────────────────────────────────────────────── */
int main(int argc, char **argv) {
    int njobs = 0;
    const char *target = NULL;

    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "-j") == 0 && i + 1 < argc) {
            njobs = atoi(argv[++i]);
        } else if (argv[i][0] != '-') {
            target = argv[i];
        }
    }

    if (!target) {
        fprintf(stderr, "Usage: fptool [-j N] <file|directory>\n");
        return 1;
    }

    use_avx2_global = has_avx2();
    if (!use_avx2_global)
        fprintf(stderr, "Note: AVX2 not available, using scalar path\n");

    if (njobs <= 0) njobs = (int)sysconf(_SC_NPROCESSORS_ONLN);
    if (njobs < 1) njobs = 1;

    struct stat st;
    if (stat(target, &st) != 0) {
        fprintf(stderr, "error: cannot stat %s\n", target);
        return 1;
    }

    if (S_ISDIR(st.st_mode)) {
        /* Parallel directory walk */
        pthread_t *threads = (pthread_t *)malloc((size_t)njobs * sizeof(pthread_t));
        for (int i = 0; i < njobs; i++)
            pthread_create(&threads[i], NULL, worker, NULL);

        walk_dir(target);

        pthread_mutex_lock(&queue_mutex);
        queue_done = 1;
        pthread_cond_broadcast(&queue_cond);
        pthread_mutex_unlock(&queue_mutex);

        for (int i = 0; i < njobs; i++)
            pthread_join(threads[i], NULL);
        free(threads);
    } else {
        /* Single file */
        fp_file(target, use_avx2_global);
    }

    return 0;
}
