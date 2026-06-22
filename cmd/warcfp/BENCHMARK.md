# warcfp Common Crawl Benchmark

**Date**: June 2026
**Source**: CC-MAIN-2026-25, segment 1780687572080.85, first 50MB compressed
**Decompressed**: 122 MB
**GPU**: Quadro RTX 4000

## Results

| Tool | Wall Time | CPU Time | Strategy |
|------|-----------|----------|----------|
| warcfp (original) | 2.447s | 1.783s | CPU weights + GPU dot product |
| warcfp_fast | 0.727s | 0.080s | GPU weights + GPU dot product |
| Speedup | 3.4× | 22.3× | |

## Fingerprint Quality

- Total records: 1,148 (one per unique response page)
- Unique fingerprints: 1,136
- Duplicates: 0 (bug fixed — stale body pointer in non-response records)

## Bug Fixed

The original code had a bug where non-response WARC records (request, metadata)
also have WARC-Target-URI headers. The URL was parsed from these records,
but the body pointer from the previous response record was not cleared.
This caused each page to be fingerprinted 3 times under 3 different URLs
(response, request, metadata). Fixed by nulling `*body_out` in all early
return paths.

## Sample Output

Full listing: `cc_sample_fingerprints.txt` (1,148 lines)
