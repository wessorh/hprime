# Montgomery Pair Correlation Test on Hilbert Plane Operator Eigenvalues
# Tests whether eigenvalue spacings follow GUE distribution (same as zeta zeros)

import numpy as np
from scipy import stats

zeta_zeros = np.array([14.134725,21.022040,25.010857,30.424876,32.935062,37.586178,40.918719,43.327073,48.005150,49.773832,52.970321,56.446248,59.347044,60.831779,65.112544,67.079810,69.546402,72.067158,75.704691,77.144840,79.337375,82.910380,84.735493,87.425275,88.809111,92.491899,94.651344,95.870634,98.831194,101.317851,103.725538,105.446623,107.168611,111.029837,111.874659,114.320221,116.226680,118.015959,121.370125,122.946829,124.256819,127.516684,129.577748,131.087689,133.497737,134.756510,138.116043,139.736209,141.123707,143.111846,146.000982,147.422765,150.053520,150.968220,153.776651,156.112607,157.597592,158.849988,161.188964,163.030709,165.537069,167.184440,169.094515,169.911976])

def gue_pcf(t):
    result = np.ones_like(t)
    nonzero = t != 0
    st = np.sin(np.pi * t[nonzero]) / (np.pi * t[nonzero])
    result[nonzero] = 1.0 - st * st
    return result

def pair_correlation_histogram(values, max_t=3.0, bins=40):
    sorted_vals = np.sort(values)
    spacings = np.diff(sorted_vals)
    mean_spacing = np.mean(spacings)
    normalized = spacings / mean_spacing
    hist, edges = np.histogram(normalized, bins=bins, range=(0, max_t), density=True)
    return edges[:-1] + (edges[1]-edges[0])/2, hist

# Run for all orders
zz_edges, zz_pcf = pair_correlation_histogram(zeta_zeros)
gue_theory = gue_pcf(zz_edges)

orders = [6, 7, 8, 9, 10]
print("Order  r(GUE)    p(GUE)    r(zeta)   p(zeta)")
for n in orders:
    ev = np.load(f'../eigenvalues_n{n}.npy')
    edges, pcf = pair_correlation_histogram(ev)
    r_gue, p_gue = stats.pearsonr(pcf, gue_theory)
    r_zz, p_zz = stats.pearsonr(pcf, zz_pcf)
    print(f"{n:<6} {r_gue:<10.4f} {p_gue:<10.4f} {r_zz:<10.4f} {p_zz:<10.4f}")
