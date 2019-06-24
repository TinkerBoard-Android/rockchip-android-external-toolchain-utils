package main

func checkUnsupportedFlags(cmd *command) error {
	for _, arg := range cmd.args {
		if arg == "-fstack-check" {
			return newUserErrorf(`option %q is not supported; crbug/485492`, arg)
		}
	}
	return nil
}
