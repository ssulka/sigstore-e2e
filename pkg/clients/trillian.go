package clients

type Trillian struct {
	*cli
}

func NewTrillianCli() *Trillian {
	return &Trillian{
		&cli{
			Name:           "trillian",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
