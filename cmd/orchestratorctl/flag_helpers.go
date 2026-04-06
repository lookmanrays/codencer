package main

import "time"

type startOptions struct {
	wait bool
	json bool
}

func parseStartOptions(args []string) startOptions {
	return startOptions{
		wait: hasFlag(args, "--wait"),
		json: hasFlag(args, "--json"),
	}
}

func parseWaitFlags(args []string) (interval, timeout time.Duration, asJSON bool) {
	interval = 2 * time.Second
	asJSON = hasFlag(args, "--json")

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--interval":
			if i+1 < len(args) {
				d, err := time.ParseDuration(args[i+1])
				if err == nil {
					interval = d
					i++
				}
			}
		case "--timeout":
			if i+1 < len(args) {
				d, err := time.ParseDuration(args[i+1])
				if err == nil {
					timeout = d
					i++
				}
			}
		}
	}

	return
}
