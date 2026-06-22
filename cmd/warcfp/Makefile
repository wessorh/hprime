.PHONY: all clean test

TARGET  = warcfp
SRCS    = warcfp.cu
NVCC    = nvcc
CFLAGS  = -O3
LDFLAGS = -lm

all: $(TARGET)

$(TARGET): $(SRCS)
	$(NVCC) $(CFLAGS) -o $@ $< $(LDFLAGS)

test: $(TARGET)
	@echo "Building test WARC..."
	@printf 'WARC/1.0\r\nWARC-Type: response\r\nWARC-Target-URI: http://example.com/\r\nContent-Length: 42\r\n\r\nHTTP/1.0 200 OK\r\n\r\n<html><body>hello world</body></html>\r\n\r\n' > /tmp/test.warc
	@./$(TARGET) /tmp/test.warc
	@rm -f /tmp/test.warc

clean:
	rm -f $(TARGET)
