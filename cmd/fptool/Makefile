.PHONY: all clean test bench distclean

CC       = gcc
CFLAGS   = -O3 -mavx2 -Wall -Wextra
LDFLAGS  = -lm -lpthread
TARGET   = fptool
BENCHBIN = bench_fp
SRCS     = main.c
BENCHSRC = benchmark.c
OBJS     = $(SRCS:.c=.o)

# holloman5 — only needed for bench target
HOLLOMAN_DIR = /home/wessorh/work/ai/holloman5
HOLLOMAN_INC = -I$(HOLLOMAN_DIR)/src/common \
               -I$(HOLLOMAN_DIR)/src/avx2/hilbert-avx2/include
HOLLOMAN_LIB = $(HOLLOMAN_DIR)/lib/libholloman_avx2.a

all: $(TARGET)

$(TARGET): $(OBJS)
	$(CC) $(CFLAGS) -o $@ $^ $(LDFLAGS)

$(BENCHBIN): $(BENCHSRC) $(HOLLOMAN_LIB)
	$(CC) $(CFLAGS) $(HOLLOMAN_INC) -o $@ $< $(HOLLOMAN_LIB) $(LDFLAGS)

bench: $(BENCHBIN)
	@echo "=== fptool vs holloman5 ==="
	@echo "Usage: ./$(BENCHBIN) <file> [file2 ...]"

%.o: %.c
	$(CC) $(CFLAGS) -c -o $@ $<

test: $(TARGET)
	@echo "Determinism test:"
	@./$(TARGET) /bin/ls > /tmp/fp1.txt
	@./$(TARGET) /bin/ls > /tmp/fp2.txt
	@diff /tmp/fp1.txt /tmp/fp2.txt && echo "PASS: deterministic" || echo "FAIL"
	@rm -f /tmp/fp1.txt /tmp/fp2.txt

clean:
	rm -f $(TARGET) $(BENCHBIN) $(OBJS)

distclean: clean
	rm -f $(HOLLOMAN_LIB)
