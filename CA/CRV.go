package CA

import (
	"CTngV2/crypto"
	"CTngV2/definition"
	"encoding/json"

	"github.com/bits-and-blooms/bitset"
)

type CRV struct {
	CRV_pre_update *bitset.BitSet
	CRV_current    *bitset.BitSet
	CRV_cache      map[string]*bitset.BitSet
}

type Revocation struct {
	Period    string
	Delta_CRV []byte
	SRH       string
}

func CRV_init() *CRV {
	CRV := new(CRV)
	CRV.CRV_pre_update = bitset.New(700)
	CRV.CRV_current = bitset.New(700)
	CRV.CRV_cache = make(map[string]*bitset.BitSet)
	return CRV
}

// Compute delta between CRV_pre_update and CRV_current
func (crv *CRV) GetDeltaCRV() []byte {
	// compute delta between CRV_pre_update and CRV_current
	CRV_delta := crv.CRV_current.SymmetricDifference(crv.CRV_pre_update)
	bytes, err := CRV_delta.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return bytes
}

// Compute delta between one of the cached CRV and CRV_current
func (crv *CRV) GetDeltaCRVCache(LastUpdatePeriod string) []byte {
	// compute delta between CRV_pre_update and CRV_current
	CRV_delta := crv.CRV_current.SymmetricDifference(crv.CRV_cache[LastUpdatePeriod])
	bytes, err := CRV_delta.MarshalBinary()
	if err != nil {
		panic(err)
	}
	return bytes
}

// revoke by revocation ID
func (crv *CRV) Revoke(index int) {
	crv.CRV_current.Set(uint(index))
}

func Generate_Revocation(c *CAContext, Period string, REV_type int) definition.Gossip_object {
	// if REV_type == 1 generate a REV based on another Delta_CRV
	// hash c.CRV.CRVcurrent
	hashmsg, _ := c.CRV.CRV_current.MarshalBinary()
	hash, _ := crypto.GenerateSHA256(hashmsg)
	var hashmsgdelta []byte
	// hash delta CRV
	if REV_type == 0 {
		hashmsgdelta = c.CRV.GetDeltaCRV()
	} else {
		// get Delta CRV from first
		m1 := c.CRV.GetDeltaCRV()
		// Unmarshal m1
		var m2 bitset.BitSet
		err := m2.UnmarshalBinary(m1)
		if err != nil {
			panic(err)
		}
		m2.Set(1)
		hashmsgdelta, err = m2.MarshalBinary()
		if err != nil {
			panic(err)
		}
	}
	hash_delta, _ := crypto.GenerateSHA256(hashmsgdelta)
	// hash Period||hash CRVcurrent||hash delta CRV
	hash_revocation, _ := crypto.GenerateSHA256([]byte(Period + string(hash) + string(hash_delta)))
	// sign hash_revocation
	signature, _ := crypto.RSASign(hash_revocation, &c.CA_crypto_config.SignSecretKey, c.CA_crypto_config.SelfID)
	// create revocation object
	revocation := Revocation{
		Period:    Period,
		Delta_CRV: hashmsgdelta,
		SRH:       signature.String(),
	}
	// create gossip object
	payload3, _ := json.Marshal(revocation)
	payload := string(c.CA_private_config.Signer) + "CRV" + string(payload3)
	sig, _ := crypto.RSASign([]byte(payload), &c.CA_crypto_config.SignSecretKey, c.CA_crypto_config.SelfID)
	gossipREV := definition.Gossip_object{
		Application:   "CTng",
		Type:          definition.REV_INIT,
		Period:        Period,
		Signer:        c.CA_private_config.Signer,
		Signature:     [2]string{sig.String(), ""},
		Crypto_Scheme: "RSA",
		Payload:       [3]string{c.CA_private_config.Signer, "CRV", string(payload3)},
	}
	return gossipREV
}
