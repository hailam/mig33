package media

type Photo struct {
	ID                   int64
	Owner, Caption, MIME string
	Data                 []byte
	CreatedAt            int64
}
