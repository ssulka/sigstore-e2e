package clients

type TrillianCli struct {
	*cli
}

func NewTrillianCli() *TrillianCli {
	return &TrillianCli{
		&cli{
			Name:           "trillian-cli",
			setupStrategy:  PreferredSetupStrategy(),
			versionCommand: "version",
		}}
}
