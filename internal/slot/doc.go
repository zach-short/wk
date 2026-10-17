// Package slot caps heavy commands machine-wide: `wk run --slot <class>` holds one of a class's
// places while its command runs, waits in arrival order when every place is taken, and passes
// the command's input, output, signals and exit status through unchanged.
//
// The capacities live in <WK_HOME>/machine.toml and nothing about a class is compiled in (D19).
// Every failure of the machinery itself runs the command unslotted, so a broken wk can delay a
// gate but never turn it red or green (SLOTS-PLAN.md SB-9). The word "slot" already names pool
// slots and create slots, so everything this package prints says "place" with its class.
package slot
