package CA

import (
	"CTngV2/crypto"
	"CTngV2/definition"
	"CTngV2/util"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
)

type CAContext struct {
	Client                 *http.Client
	SerialNumber           int
	CA_public_config       *CA_public_config
	CA_private_config      *CA_private_config
	CA_crypto_config       *crypto.CryptoConfig
	PublicKey              rsa.PublicKey
	PrivateKey             rsa.PrivateKey
	CurrentCertificatePool *crypto.CertPool
	CurrentKeyPool         map[string]*rsa.PrivateKey
	CertPoolStorage        *CTngCertPoolStorage
	Rootcert               *x509.Certificate
	CertCounter            int
	CRV                    *CRV
	CA_Type                int                                 //0 for normal CA, 1 for Split-world CA, 2 for always unreponsive CA, 3 for sometimes unreponsive CA
	Request_Count          int                                 //Only used for sometimes unreponsive CA and Split-world CA
	OnlineDuration         int                                 //Only used for sometimes unreponsive CA and Split-world CA
	REV_storage            map[string]definition.Gossip_object //for monitor to query
	REV_storage_fake       map[string]definition.Gossip_object //for monitor to query
	MisbehaviorInterval    int                                 //for sometimes unreponsive CA and Split-world CA, misbehave every x requests
	StoragePath1           string
	StoragePath2           string
	STH_storage            map[string]definition.Gossip_object //store the STH by LID
	Request_Count_lock     *sync.Mutex
}

type CA_public_config struct {
	All_CA_URLs     []string
	All_Logger_URLs []string
	MMD             int
	MRD             int
	Http_vers       []string
}

type CA_private_config struct {
	Signer          string
	Port            string
	Loggerlist      []string
	Monitorlist     []string
	Gossiperlist    []string
	Cert_per_period int
}

type ProofOfInclusion struct {
	SiblingHashes [][]byte `json:"sibling_hashes,omitempty"` // SiblingHashes is the list of sibling hashes in the Merkle tree
	NeighborHash  []byte   `json:"neighbor_hash,omitempty"`  // NeighborHash is the hash of the neighbor node in the Merkle tree
	LoggerID      string   `json:"LoggerID,omitempty"`       // LoggerID is the ID of the CT log
	SubjectKeyId  []byte   `json:"SubjectKeyId,omitempty"`   // SubjectKeyId is the Subject Key Identifier of the certificate
}

// RID is self generated by the CA
type LoggerInfo struct {
	STH definition.Gossip_object `json:"STH,omitempty"` // STH is the Signed Tree Head of the CT log
	POI ProofOfInclusion         `json:"POI,omitempty"` // POI is the proof of inclusion of the certificate in the CT log
}

type SequenceNumber struct {
	RID int `json:"RID,omitempty"`
}

type CTngExtension struct {
	SequenceNumber    SequenceNumber `json:"SequenceNumber,omitempty"`
	LoggerInformation []LoggerInfo   `json:"LoggerInformation,omitempty"`
}

var (
	OIDCTngExtension = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 67847871}
)

type CTngCertPoolStorage struct {
	Certpools map[string]crypto.CertPool
}

func EncodeCTngExtension(ctngext CTngExtension) []byte {
	// encode the ctngext to bytes using json marshal
	ctngextbytes, err := json.Marshal(ctngext)
	if err != nil {
		fmt.Println("Error in EncodeCTngExtension: ", err)
	}
	// encode the json bytes to asn1 bytes
	OctectString := asn1.RawValue{
		Tag:   asn1.TagOctetString,
		Bytes: ctngextbytes,
	}
	ctngextasn1bytes, err := asn1.Marshal(OctectString)
	if err != nil {
		fmt.Println("Error in EncodeCTngExtension: ", err)
	}
	return ctngextasn1bytes
}

func DecodeCTngExtension(ctngextasn1bytes []byte) CTngExtension {
	// decode the asn1 bytes to json bytes
	var OctectString asn1.RawValue
	_, err := asn1.Unmarshal(ctngextasn1bytes, &OctectString)
	if err != nil {
		fmt.Println("Error in DecodeCTngExtension: ", err)
	}
	var ctngext CTngExtension
	// decode the json bytes to ctngext
	err = json.Unmarshal(OctectString.Bytes, &ctngext)
	if err != nil {
		fmt.Println("Error in DecodeCTngExtension: ", err)
	}
	return ctngext
}

func UpdateCTngExtension(cert *x509.Certificate, newloggerinfo LoggerInfo) *x509.Certificate {
	CTngExtension := ParseCTngextension(cert)
	for _, loggerinfo := range CTngExtension.LoggerInformation {
		if loggerinfo.STH.Signer == newloggerinfo.STH.Signer {
			return cert
		}
	}
	CTngExtension.LoggerInformation = append(CTngExtension.LoggerInformation, newloggerinfo)
	// Swap the old extension with the new extension
	for i, ext := range cert.Extensions {
		if ext.Id.Equal(OIDCTngExtension) {
			cert.Extensions[i].Value = EncodeCTngExtension(CTngExtension)
		}
	}
	return cert
}

func UpdateforSigning(cert *x509.Certificate) *x509.Certificate {
	CTngExtension := ParseCTngextension(cert)
	new_ext := pkix.Extension{
		Id:       OIDCTngExtension,
		Critical: false,
		Value:    EncodeCTngExtension(CTngExtension),
	}
	cert.ExtraExtensions = append(cert.ExtraExtensions, new_ext)
	return cert
}

func UpdateAllforSigning(certs []*x509.Certificate) []*x509.Certificate {
	for i, cert := range certs {
		certs[i] = UpdateforSigning(cert)
	}
	return certs
}

func ParseCTngextension(cert *x509.Certificate) CTngExtension {
	var ctngext CTngExtension
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(OIDCTngExtension) {
			ctngext = DecodeCTngExtension(ext.Value)
		}
	}
	return ctngext
}

func GetRIDfromCert(cert *x509.Certificate) int {
	// Parse the CTng extension first
	ext := ParseCTngextension(cert)
	return ext.SequenceNumber.RID
}

func GetPrecertfromCert(cert *x509.Certificate) *x509.Certificate {
	// Parse the CTng extension first
	ext := ParseCTngextension(cert)
	// now create a new CTng extension with only the sequence number field
	var newCTngExtension CTngExtension
	newCTngExtension.LoggerInformation = []LoggerInfo{}
	newCTngExtension.SequenceNumber = ext.SequenceNumber
	precert := util.ParseTBSCertificate(cert)
	precert.Extensions = []pkix.Extension{
		{
			Id:       OIDCTngExtension,
			Critical: false,
			Value:    EncodeCTngExtension(newCTngExtension),
		},
	}
	return precert

}

// Generate a CA public config template
func GenerateCA_public_config_template() *CA_public_config {
	return &CA_public_config{
		All_CA_URLs:     []string{},
		All_Logger_URLs: []string{},
		MMD:             0,
		MRD:             0,
		Http_vers:       []string{},
	}
}

// Generate a CA Crypto config template
func GenerateCA_Crypto_config_template() *crypto.StoredCryptoConfig {
	return &crypto.StoredCryptoConfig{
		SelfID:             crypto.CTngID("0"),
		Threshold:          0,
		N:                  0,
		HashScheme:         0,
		SignScheme:         "",
		ThresholdScheme:    "",
		SignPublicMap:      crypto.RSAPublicMap{},
		SignSecretKey:      rsa.PrivateKey{},
		ThresholdPublicMap: map[string][]byte{},
		ThresholdSecretKey: []byte{},
	}
}

// Generate a CA private config template
func GenerateCA_private_config_template() *CA_private_config {
	return &CA_private_config{
		Signer:          "",
		Port:            "",
		Loggerlist:      []string{},
		Monitorlist:     []string{},
		Gossiperlist:    []string{},
		Cert_per_period: 0,
	}
}

// Generate a public key from a private key
func publicKey(priv *rsa.PrivateKey) rsa.PublicKey {
	return priv.PublicKey
}

// Gererate RSA key pair
func GenerateRSAKeyPair() (rsa.PrivateKey, rsa.PublicKey) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println("rsa keygen failed")
	}
	pub := publicKey(priv)
	return *priv, pub
}

// write a CA private config or ca public config or crypto config to file
func WriteConfigToFile(config interface{}, filepath string) {
	util.CreateFile(filepath)
	// file name should be "CA_private_config.json" or "CA_public_config.json" or "CA_crypto_config.json" depending on the config type
	filename := filepath + fmt.Sprintf("%T.json", config)
	jsonConfig, err := json.Marshal(config)
	if err != nil {
		fmt.Println("json marshal failed")
	}
	err = ioutil.WriteFile(filename, jsonConfig, 0644)
	if err != nil {
		fmt.Println("write file failed")
	}
}

func (ctx *CAContext) SaveToStorage() {
	path1 := ctx.StoragePath1
	path2 := ctx.StoragePath2
	data1 := [][]any{}
	data2 := [][]any{}
	//fmt.Println(path)
	certs := ctx.CurrentCertificatePool.GetCerts()
	signed_certs := SignAllCerts(ctx)

	// iterate through all certs
	for _, cert := range signed_certs {
		ext := ParseCTngextension(cert)
		data1_json, _ := json.Marshal(ext)
		data1 = append(data1, []any{data1_json})
		rid := GetRIDfromCert(cert)
		util.SaveCertificateToDisk(cert.Raw, cert.Subject.CommonName+"_RID_"+strconv.Itoa(rid)+".crt")
		util.SaveKeyToDisk(ctx.CurrentKeyPool[cert.Subject.CommonName], cert.Subject.CommonName+"_RID_"+strconv.Itoa(rid)+".key")
	}
	for _, cert := range certs {
		tbscert := util.ParseTBSCertificate(&cert)
		tbscert_json, _ := json.Marshal(tbscert)
		data2 = append(data2, []any{tbscert_json})
	}
	//write to file
	util.CreateFile(path1)
	util.CreateFile(path2)
	util.WriteData(path1, data1)
	util.WriteData(path2, data2)
}

// initialize CA context
func InitializeCAContext(public_config_path string, private_config_file_path string, crypto_config_path string) *CAContext {
	// Load public config from file
	pubconf := new(CA_public_config)
	util.LoadConfiguration(&pubconf, public_config_path)
	// Load private config from file
	privconf := new(CA_private_config)
	util.LoadConfiguration(&privconf, private_config_file_path)
	// Load crypto config from file
	cryptoconfig, err := crypto.ReadCryptoConfig(crypto_config_path)
	if err != nil {
		//fmt.Println(err)
	}
	// Initialize CA Context
	caContext := &CAContext{
		SerialNumber:           0,
		CA_public_config:       pubconf,
		CA_private_config:      privconf,
		CA_crypto_config:       cryptoconfig,
		PublicKey:              cryptoconfig.SignPublicMap[cryptoconfig.SelfID],
		PrivateKey:             cryptoconfig.SignSecretKey,
		CurrentCertificatePool: crypto.NewCertPool(),
		CertPoolStorage:        &CTngCertPoolStorage{Certpools: make(map[string]crypto.CertPool)},
		CA_Type:                0,
		Request_Count:          0,
		OnlineDuration:         0,
		REV_storage:            make(map[string]definition.Gossip_object),
		REV_storage_fake:       make(map[string]definition.Gossip_object),
		MisbehaviorInterval:    0,
		CertCounter:            0,
		STH_storage:            make(map[string]definition.Gossip_object),
		Request_Count_lock:     &sync.Mutex{},
	}
	// Initialize http client
	tr := &http.Transport{}
	caContext.Client = &http.Client{
		Transport: tr,
	}
	// Generate root certificate
	caContext.Rootcert = Generate_Root_Certificate(caContext)
	newCRV := CRV_init()
	caContext.CRV = newCRV
	return caContext
}
