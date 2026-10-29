package community

type Community struct {
	ID                       int64
	Name, Description, Owner string
	Private                  bool
	Members                  int
}

type CommunityMember struct{ Username, Role string }

type CommunityPost struct {
	ID, GroupID, ParentID, CreatedAt int64
	Author, Body                     string
}
