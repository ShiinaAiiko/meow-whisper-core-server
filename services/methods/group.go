package methods

import (
	"github.com/ShiinaAiiko/meow-whisper-core/protos"
	"github.com/cherrai/nyanyago-utils/nstrings"
	sso "github.com/cherrai/saki-sso-go"
	"github.com/jinzhu/copier"
)

func FormatGroupMembers(gm *protos.GroupMembers, users []*sso.SimpleAnonymousUserInfo) {
	for i, j := 0, len(users)-1; i <= j; i, j = i+1, j-1 {
		if FormatGroupMembersSimpleAnonymousUserInfo(gm, users[i]) {
			break
		}
		if FormatGroupMembersSimpleAnonymousUserInfo(gm, users[j]) {
			break
		}
	}
}

func FormatGroupMembersSimpleAnonymousUserInfo(gm *protos.GroupMembers, user *sso.SimpleAnonymousUserInfo) bool {
	if user.Uid == gm.AuthorId {
		sa := new(protos.SimpleAnonymousUserInfo)
		copier.Copy(sa, user)
		sa.Letter = nstrings.GetLetter(sa.Nickname)
		gm.UserInfo = sa
		return true
	}
	return false
}
