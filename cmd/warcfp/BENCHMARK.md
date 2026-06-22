# warcfp Common Crawl Benchmark

**Date**: June 2026
**Source**: CC-MAIN-2026-25, segment 1780687572080.85, first 50MB compressed
**Decompressed**: 122 MB
**GPU**: Quadro RTX 4000

## Results

| Tool | Wall Time | CPU Time | Strategy |
|------|-----------|----------|----------|
| warcfp (original) | 2.447s | 1.783s | CPU weights + GPU dot product |
| warcfp_fast | 0.916s | 0.246s | GPU weights + GPU dot product |
| Speedup | 2.7× | 7.2× | |

## Fingerprint Quality

- Total records: 3,444
- Unique fingerprints: 1,138
- Unique URLs: ~1,100
- Fingerprint match (GPU vs CPU): 1,134/1,138 (99.6%)

## Sample Output

Full listing: `cc_sample_fingerprints.txt` (3,444 lines, 32-char hex fingerprint + URL)
