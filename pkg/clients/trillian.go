package clients

type TrillianCli struct {
	*cli
}

func NewTrillianCli() *TrillianCli {
	return &TrillianCli{
		&cli{
			Name:           "trillian",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
