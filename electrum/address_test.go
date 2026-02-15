package electrum_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zauberhaus/go-electrum/electrum"
	"testing"
)

func TestAddressToElectrumScriptHash(t *testing.T) {
	tests := []struct {
		address        string
		wantScriptHash string
		fail           bool
	}{
		{
			address:        "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
			wantScriptHash: "8b01df4e368ea28f8dc0423bcf7a4923e3a12d307c875e47a0cfbf90b5c39161",
		},
		{
			address:        "34xp4vRoCGJym3xR7yCVPFHoCNxv4Twseo",
			wantScriptHash: "2375f2bbf7815e3cdc835074b052d65c9b2f101bab28d37250cc96b2ed9a6809",
		},
		{
			// bech32
			address:        "bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq",
			wantScriptHash: "f8d3a7f6141fb7c08d0fc5597dcf754093a908cafcbe4c17cedbbe91ff41f39c",
		},

		{
			address: "invalid-address",
			fail:    true,
		},
	}

	for _, tc := range tests {
		scriptHash, err := electrum.AddressToElectrumScriptHash(tc.address)
		if tc.fail {
			require.Error(t, err)
			continue
		}

		require.NoError(t, err)
		assert.Equal(t, tc.wantScriptHash, scriptHash)
	}
}
