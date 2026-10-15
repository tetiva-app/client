package localpath

import "testing"

func TestIsRemote(t *testing.T) {
	for path, remote := range map[string]bool{
		`\\fileserver\share\api.graphql`:                         true,
		`\\?\UNC\fileserver\share\api.graphql`:                   true,
		`\\?\GLOBALROOT\Device\Mup\fileserver\share\api.graphql`: true,
		`\\.\pipe\schema`:                                        true,
		`\\.\C:\schemas\api.graphql`:                             true,
		`\??\UNC\fileserver\share\api.graphql`:                   true,
		`\??\GLOBALROOT\Device\Mup\fileserver\share\api.graphql`: true,
		`\??\C:\schemas\api.graphql`:                             true,
		`/??/UNC/fileserver/share/api.graphql`:                   true,
		`/??\UNC\fileserver\share\api.graphql`:                   true,
		`//fileserver/share/api.graphql`:                         true,
		`/\fileserver\share\api.graphql`:                         true,
		`smb://fileserver/share/api.graphql`:                     true,
		`file:///Users/me/api.graphql`:                           true,
		`https://example.com/schema.graphql`:                     true,
		`\\?\C:\schemas\api.graphql`:                             false,
		`\\?\c:/schemas/api.graphql`:                             false,
		`C:\schemas\api.graphql`:                                 false,
		`C:/schemas/api.graphql`:                                 false,
		`/Users/me/schemas/api.graphql`:                          false,
		`/?weird/api.graphql`:                                    false,
		`/?/api.graphql`:                                         false,
		`/??weird/api.graphql`:                                   false,
		`schemas/api.proto`:                                      false,
		`/Users/me/odd://name/api.proto`:                         false,
		`/Users/me/schemas/`:                                     false,
	} {
		if got := IsRemote(path); got != remote {
			t.Errorf("IsRemote(%q) = %v, want %v", path, got, remote)
		}
	}
}
