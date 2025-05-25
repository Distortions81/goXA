package main

type BitFlags uint64

// Set sets the specified bit(s) in the flags.
func (f *BitFlags) Set(flag BitFlags) {
	*f |= flag
}

// Clear unsets the specified bit(s) in the flags.
func (f *BitFlags) Clear(flag BitFlags) {
	*f &^= flag // AND NOT
}

// Toggle flips the specified bit(s) in the flags.
func (f *BitFlags) Toggle(flag BitFlags) {
	*f ^= flag
}

// Has checks if the specified bit(s) are set.
func (f BitFlags) IsSet(flag BitFlags) bool {
	return f&flag == flag
}

// Has checks if the specified bit(s) are set.
func (f BitFlags) IsNotSet(flag BitFlags) bool {
	return f&flag != flag
}

func showFeatures(flags BitFlags) {
	flagStr := ""
	for x := 0; 1<<x < fTop; x++ {
		if flags.IsSet(1 << x) {
			if flagStr != "" {
				flagStr = flagStr + ", "
			}
			flagStr = flagStr + flagNames[x]
		}
	}
	if flagStr != "" {
		doLog(false, "Archive Flags: %v", flagStr)
	}
}
