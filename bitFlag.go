package goxa

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

// IsSet checks if the specified bit(s) are set.
func (f BitFlags) IsSet(flag BitFlags) bool {
	return f&flag == flag
}

// IsNotSet checks if the specified bit(s) are not set.
func (f BitFlags) IsNotSet(flag BitFlags) bool {
	return f&flag != flag
}

func showFeatures(flags BitFlags) {
	flagStr := ""
	for x := 0; 1<<x < fTop; x++ {
		if flags.IsSet(1 << x) {
			if flagStr != "" {
				flagStr += ", "
			}
			flagStr += flagNames[x]
		}
	}
	if flagStr != "" {
		doLog(false, "Archive Flags: %v (%s)", flagStr, flagLetters(flags))
	}
}

func flagLetters(flags BitFlags) string {
	out := ""
	if flags.IsSet(fAbsolutePaths) {
		out += "a"
	}
	if flags.IsSet(fPermissions) {
		out += "p"
	}
	if flags.IsSet(fModDates) {
		out += "m"
	}
	if flags.IsSet(fChecksums) {
		out += "s"
	}
	if flags.IsSet(fBlockChecksums) {
		out += "b"
	}
	if flags.IsSet(fNoCompress) {
		out += "n"
	}
	if flags.IsSet(fIncludeInvis) {
		out += "i"
	}
	if flags.IsSet(fSpecialFiles) {
		out += "o"
	}
	return out
}

// flagNamesList returns a slice of human-readable flag names.
func flagNamesList(flags BitFlags) []string {
	var out []string
	for x := 0; 1<<x < fTop; x++ {
		if flags.IsSet(1 << x) {
			out = append(out, flagNames[x])
		}
	}
	return out
}
