# Why You Can't Hilbert Your Way to an RH Proof

Short answer: you can't. But the setup gets close enough that it takes real
work to find the exact reason it can't — and that reason turns out to be
more interesting than the failed attempt itself.

## The setup

The Riemann Hypothesis says every non-trivial zero of the zeta function
sits on a specific vertical line in the complex plane. Nobody has proven
it, but there's a beautiful conjectural shortcut called the
Hilbert–Pólya program: if you could find some self-adjoint operator — the
kind of thing that shows up constantly in quantum mechanics, always with
real eigenvalues — whose eigenvalues happened to be exactly the zeta
zeros, the Hypothesis would fall out for free. Self-adjoint operators
can't have complex eigenvalues, so if the zeros are a spectrum, they're
automatically on the line.

The catch, for over a century, has been finding the operator. Nobody has
one. But the conjecture is specific enough that you can go looking, and
people have. This project is one of those searches, built on an unusually
elegant piece of geometry: the Hilbert curve.

A Hilbert curve is a way of snaking a single path through every cell of a
grid — 2D, 3D, or higher — so that consecutive integers always land on
touching cells. It's a real, well-studied mathematical object, mostly
famous for spatial indexing (databases use it to keep nearby data physically
nearby on disk). The idea here: color the cells by whether their position
along the curve is a prime number, slice the grid into layers, and measure
how much each layer's prime density correlates with its neighbors. That
gives you a matrix. Matrices have eigenvalues. Do the eigenvalues look
like zeta zeros?

They do. Suspiciously well.

## The suspiciously good part

Take the first 64 known zeta zeros — real numbers, computed to high
precision, publicly available. Take the eigenvalues of this Hilbert-curve
matrix, sorted by size. Line them up and compute a correlation coefficient.
You get numbers like −0.99. In statistics, that's about as strong as
correlation gets. A properly randomized permutation test (shuffle the
eigenvalues at random 20,000 times, see how often you'd get a match this
good by chance) says: basically never. This isn't noise.

And there's a genuine, provable piece of mathematics sitting underneath
it. If you build the layer-comparison matrix using true spatial
adjacency — literally, which grid cells physically touch — instead of
just following the curve's path, something clean happens: the matrix
becomes *exactly* tridiagonal. Every entry more than one step off the main
diagonal is precisely zero, not approximately, to floating-point
precision, for every dimension and every size tested. That's a real
theorem with a one-paragraph proof: a face-touching neighbor changes
exactly one coordinate by exactly one step, so it can only ever land you
in an adjacent layer or the same one, never two layers away. Nothing
about primes is needed for that part — it's just geometry.

And tridiagonal matrices with constant diagonals have been fully solved
since the 1950s. Their eigenvalues are a simple cosine formula. You don't
even need a computer to diagonalize them — you can write down the answer.
So this Hilbert curve operator isn't just numerically close to the zeta
zeros; it has an exact, closed-form spectrum, and that spectrum is what's
correlating at −0.99.

## Where it falls apart

Here's the thing about a cosine. It oscillates between a fixed high point
and a fixed low point, forever. No matter how big you make the matrix —
double it, square it, go to a million dimensions — the eigenvalues stay
trapped in the same bounded range. You just get more of them, packed more
tightly into the same box.

The zeta zeros do not do this. They keep growing, forever, and the rate
they grow at is known precisely — it's roughly *(T/2π)log(T/2π)*, a
formula proven a century ago. It's not just that the zeros get bigger; the
*spacing* between them shrinks in a very specific, logarithmic way as you
go further out.

A bounded, oscillating spectrum and an unbounded, log-growing spectrum are
different *kinds* of thing. You cannot rescale one into the other with a
multiplier, a polynomial, or any calibration curve, because rescaling
preserves boundedness. If your sequence lives in a box, no formula turns
it into a sequence that runs off to infinity. This is the actual reason
the correlation, however strong, can never become an equality. Not "we
haven't found the right scaling yet" — a proof that no scaling exists.

The −0.99 correlation, it turns out, is really a curve-fitting accident:
a cosine and a *k log k*-shaped curve happen to look a lot alike over a
short enough stretch — which the first 64 zeros are. Taylor-expand the
cosine near its peak and you get something quadratic; the zeta counting
function, over that same short range, is *also* well-approximated by
something close to quadratic. Two different functions, briefly wearing
the same disguise.

## Not the first time

This is, gratifyingly, not a unique or embarrassing failure — it's a
recognizable *type* of failure, and other serious attempts hit variations
of it. In 1993, physicists built a one-dimensional quantum potential,
via genuine inverse-scattering mathematics, whose energy levels reproduce
the first five hundred zeta zeros almost exactly. The catch: they admit
outright that you can fit *any* finite stretch of zeros this way by tuning
the potential further — it's a fit to a known answer, not a derivation
from one. Berry and Keating's famous 1999 proposal, the H=xp model, does
get the *growth rate* right, by a genuine physics argument about phase
space volume — but the operator itself isn't rigorously well-defined
without extra assumptions nobody's proven. Connes' noncommutative-geometry
approach sidesteps the whole density problem with a much heavier
machinery, and lands the Riemann Hypothesis exactly where it reduces to
another open conjecture. Every serious attempt seems to get exactly one
or two of the three things it needs — correct density, rigorous operator,
actual proof — and never all three at once.

## Why bother, then

Because the process of finding out *exactly* why something plausible
fails is not the same as it just failing. You come out the other side
with a real theorem (the tridiagonal structure, useful independently as a
cheap way to verify a space-filling curve implementation isn't secretly
broken — which, during this project, it was, twice), a precise criterion
anyone else can check their own candidate operator against before sinking
months into it, and a genuinely unrelated engineering payoff: realizing
*why* the covariance matrix had this structure led to a much faster
fingerprinting algorithm for an unrelated piece of production software,
with no Riemann zeta function in sight.

The Riemann Hypothesis is still open. This didn't move the needle on it.
But it drew one clean, small circle around a place a proof cannot live,
and that's a legitimate thing for a piece of math to do, even when the
headline is "no."
