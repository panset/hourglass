// SPDX-License-Identifier: MIT

//go:build race

package api

// raceEnabled reports that the race detector is on, which slows the code under
// test by an order of magnitude: wall-clock bounds are then not enforced.
const raceEnabled = true
