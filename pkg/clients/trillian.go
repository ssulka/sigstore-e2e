package clients

type Trillian struct {
	*cli
}

func NewTrillian() *Trillian {
	return &Trillian{
		&cli{
			Name:           "trillian",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
