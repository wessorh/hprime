.PHONY: all clean test

NVCC    = nvcc
CFLAGS  = -O3
LDFLAGS = -lm
TARGETS = warcfp warcfp_fast

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
