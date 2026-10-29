package activity

type ServiceEntry struct {
	ID                                      int64
	Kind, Owner, Target, Body, State, Nonce string
	CreatedAt                               int64
}
