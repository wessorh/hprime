#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <math.h>
#include <time.h>
#include <sys/stat.h>
#include <immintrin.h>
#include "holloman.h"

#define FP 16
#define BITS 14
#define SCALE (1<<BITS)
#define ROUND (1<<(BITS-1))
#define PI  3.141592653589793
#define MAX_TAPS 2048

static double l3(double x){
    if(x>3.0)return 0;if(x==0)return 1;
    double b=x*PI,c=b/3.0;return sin(b)*sin(c)/(b*c);
}
static uint8_t cl8(int x){return x<0?0:(x>255?255:(uint8_t)x);}

typedef struct{int32_t off;int32_t n;int16_t w[MAX_TAPS];}K1D;

static void k1d_build(K1D *k, size_t ilen){
    double flen=(double)ilen;
    for(int i=0;i<FP;i++){
        double ctr=(i+0.5)*flen/FP,sup=3.0*flen/FP;
        int L=(int)ceil(ctr-sup),R=(int)floor(ctr+sup);
        if(L<0)L=0;if(R>=(int)ilen)R=(int)ilen-1;
        int n=R-L+1;
        if(n>MAX_TAPS){int x=(n-MAX_TAPS)/2;L+=x;R=L+MAX_TAPS-1;n=MAX_TAPS;}
        k[i].off=L;k[i].n=n;
        double sum=0,ws[MAX_TAPS];
        for(int j=L;j<=R;j++){ws[j-L]=l3(fabs((ctr-j)*FP/flen));sum+=ws[j-L];}
        double sw=SCALE/sum;
        for(int j=0;j<n;j++){double w=ws[j]*sw;k[i].w[j]=(int16_t)(w<0?w-0.5:w+0.5);}
    }
}

static void fptool_dot(const uint8_t *d,const K1D *k,uint8_t out[FP]){
    for(int i=0;i<FP;i++){
        int L=k[i].off,n=k[i].n;
        const uint8_t *p=d+L;const int16_t *w=k[i].w;
        __m256i acc=_mm256_setzero_si256();int j=0;
        for(;j+8<=n;j+=8){
            __m128i pu=_mm_loadl_epi64((const __m128i*)&p[j]);
            __m256i pi=_mm256_cvtepu8_epi32(pu);
            __m128i wi=_mm_loadu_si128((__m128i*)&w[j]);
            __m256i wii=_mm256_cvtepi16_epi32(wi);
            acc=_mm256_add_epi32(acc,_mm256_mullo_epi32(pi,wii));
        }
        __m128i lo=_mm256_castsi256_si128(acc),hi=_mm256_extracti128_si256(acc,1);
        __m128i s=_mm_add_epi32(lo,hi);s=_mm_hadd_epi32(s,s);s=_mm_hadd_epi32(s,s);
        int32_t sum=_mm_cvtsi128_si32(s);
        for(;j<n;j++)sum+=(int32_t)p[j]*(int32_t)w[j];
        out[i]=cl8((sum+ROUND)>>BITS);
    }
}

static double ms(void){struct timespec t;clock_gettime(CLOCK_MONOTONIC,&t);return t.tv_sec*1000.0+t.tv_nsec/1e6;}
static double l2dist(const uint8_t *a,const uint8_t *b){
    double s=0;for(int i=0;i<FP;i++){double d=a[i]-b[i];s+=d*d;}return sqrt(s);
}

static uint8_t *rf(const char *path,size_t *len){
    FILE*f=fopen(path,"rb");if(!f)return NULL;
    fseek(f,0,SEEK_END);*len=ftell(f);fseek(f,0,SEEK_SET);
    uint8_t *d=malloc(*len?*len:1);
    if(*len){size_t nr=fread(d,1,*len,f);(void)nr;}fclose(f);return d;
}

int main(int argc,char **argv){
    if(argc<2){fprintf(stderr,"Usage: %s <file> [file2 ...]\n",argv[0]);return 1;}

    printf("%-8s %12s %12s %8s %8s %8s\n",
           "Size","fptool_ns","holloman_ns","Speedup","L2dist","Match?");
    printf("%-8s %12s %12s %8s %8s %8s\n",
           "--------","----------","-----------","-------","------","------");

    for(int a=1;a<argc;a++){
        size_t len;uint8_t *data=rf(argv[a],&len);
        if(!data){fprintf(stderr,"skip %s\n",argv[a]);continue;}

        /* Precompute kernel ONCE (outside timing) — heap to avoid stack crash */
        K1D *k = (K1D *)malloc(FP * sizeof(K1D));
        if (!k) { free(data); continue; }
        k1d_build(k,len);

        /* holloman5 init */
        int r=holloman_init(13);
        if(r!=0){fprintf(stderr,"h5 init fail %d\n",r);free(data);continue;}

        uint8_t fp1[FP],fp2[FP];
        int niter = len < 16384 ? 1000 : (len < 65536 ? 500 : 200);

        /* fptool benchmark: kernel precomputed, just dot product */
        double t0=ms();
        for(int i=0;i<niter;i++)fptool_dot(data,k,fp1);
        double t1=ms();

        /* holloman5 benchmark */
        double t2=ms();
        for(int i=0;i<niter;i++){
            r=holloman_fingerprint_buffer(data,len,fp2);
            if(r!=0){fprintf(stderr,"h5 fp fail iter %d: %d\n",i,r);break;}
        }
        double t3=ms();

        double ns1=(t1-t0)/niter*1e6,ns2=(t3-t2)/niter*1e6;
        double l2=l2dist(fp1,fp2);
        int match=(memcmp(fp1,fp2,FP)==0);

        char ss[16];
        if(len<1024)sprintf(ss,"%zuB",len);
        else if(len<1048576)sprintf(ss,"%zuKB",len/1024);
        else snprintf(ss,sizeof(ss),"%zuMB",len/1048576);

        printf("%-8s %10.0f ns %10.0f ns %7.1fx %8.2f %8s\n",
               ss,ns1,ns2,ns2/ns1,l2,match?"YES":"NO");
        printf("  fptool=");
        for(int i=0;i<FP;i++)printf("%02x",fp1[i]);
        printf(" h5=");
        for(int i=0;i<FP;i++)printf("%02x",fp2[i]);
        printf("\n");

        free(data);free(k);holloman_cleanup();
    }
    return 0;
}
