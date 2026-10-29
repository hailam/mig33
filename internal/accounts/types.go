package accounts

type Identity struct {
	ID                                              int32
	Username, Display, Status, Mobile, Bio, Country string
	Activated                                       bool
	PasswordSalt, PasswordKey                       []byte
	PasswordIterations                              int
	LegacyHash                                      int32
	LegacyUnits                                     int
}

type IdentityChallenge struct {
	Token, Purpose, Username, Mobile string
	ExpiresAt                        int64
	Attempts                         int
}
