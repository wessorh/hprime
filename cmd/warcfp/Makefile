.PHONY: all clean test

NVCC    = nvcc

# Fat binary covering Turing through Blackwell (CUDA 13.3 minimum: sm_75)
# For Tesla P4 (Pascal, sm_61) with CUDA 11.x/12.x + glibc >= 2.40:
#   nvcc -O3 -arch=compute_61 -code=sm_61 --std=c++14 -D__NO_MATH_INLINES \
#        -o warcfp_fast warcfp_fast.cu -lm
# The --std=c++14 + -D__NO_MATH_INLINES fixes a noexcept mismatch between
# CUDA's math_functions.h and glibc 2.40's mathcalls.h (cospi/sinpi).
ARCHFLAGS = -gencode arch=compute_75,code=sm_75 \
            -gencode arch=compute_80,code=sm_80 \
            -gencode arch=compute_86,code=sm_86 \
            -gencode arch=compute_89,code=sm_89 \
            -gencode arch=compute_90,code=sm_90

CFLAGS   = -O3 $(ARCHFLAGS)
# -Xcompiler -fno-exceptions fixes noexcept mismatch on CUDA < 13 with glibc >= 2.40
# Only needed for Pascal (sm_61) builds. Leave commented for default builds.
# CFLAGS += -Xcompiler -fno-exceptions
LDFLAGS  = -lm
TARGETS  = warcfp warcfp_fast

all: $(TARGETS)

warcfp: warcfp.cu
	$(NVCC) $(CFLAGS) -o $@ $< $(LDFLAGS)

warcfp_fast: warcfp_fast.cu
	$(NVCC) $(CFLAGS) -o $@ $< $(LDFLAGS)

test: warcfp
	@printf 'WARC/1.0\r\nWARC-Type: response\r\nWARC-Target-URI: http://example.com/\r\nContent-Length: 45\r\n\r\nHTTP/1.0 200 OK\r\n\r\n<html><body>hello</body></html>\r\n\r\n' > /tmp/test.warc
	@./warcfp /tmp/test.warc
	@rm -f /tmp/test.warc

clean:
	rm -f $(TARGETS)
