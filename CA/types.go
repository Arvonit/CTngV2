package CA

import (
	"CTngV2/crypto"
	"CTngV2/definition"
	"CTngV2/util"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
	StoragePath            string
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
	SiblingHashes [][]byte
	NeighborHash  []byte
}

type POI struct {
	ProofOfInclusion ProofOfInclusion
	SubjectKeyId     []byte
	LoggerID         string
}

//RID is self generated by the CA
type CTngExtension struct {
	STH definition.Gossip_object `json:"STH,omitempty"` // STH is the Signed Tree Head of the CT log
	POI ProofOfInclusion         `json:"POI,omitempty"` // POI is the proof of inclusion of the certificate in the CT log
}

type SequenceNumber struct {
	RID int `json:"RID"`
}

type CTngCertPoolStorage struct {
	Certpools map[string]crypto.CertPool
}

// add CTngExtension to a certificate
func AddCTngExtension(cert *x509.Certificate, ctngext CTngExtension) *x509.Certificate {
	// use CRLDistributionPoints to store the CTngExtension
	// encode the ctngext to bytes using json marshal indent
	ctngextbytes, err := json.MarshalIndent(ctngext, "", "  ")
	if err != nil {
		fmt.Println("Error in AddCTngExtension: ", err)
	}
	// encode the bytes to string
	ctngextstr := string(ctngextbytes)
	// add the ctngext to the cert
	cert.CRLDistributionPoints = append(cert.CRLDistributionPoints, ctngextstr)
	return cert
}

// get all ctng extensions from a certificate
func GetCTngExtensions(cert *x509.Certificate) []any {
	// use CRLDistributionPoints to store the CTngExtension
	var ctngexts []any
	// parse cert.CRLDistributionPoints[0] to get the sequence number
	var ctngext SequenceNumber
	// convert the string to bytes
	ctngextbytes := []byte(cert.CRLDistributionPoints[0])
	// decode the bytes to ctngext
	err := json.Unmarshal(ctngextbytes, &ctngext)
	if err != nil {
		fmt.Println("Error in GetCTngExtensions: ", err)
	}
	ctngexts = append(ctngexts, ctngext)
	if len(cert.CRLDistributionPoints) == 1 {
		return ctngexts
	} else {
		for _, ext := range cert.CRLDistributionPoints[1:] {
			var ctngext CTngExtension
			// convert the string to bytes
			ctngextbytes := []byte(ext)
			// decode the bytes to ctngext
			err := json.Unmarshal(ctngextbytes, &ctngext)
			if err != nil {
				fmt.Println("Error in GetCTngExtensions: ", err)
			}
			ctngexts = append(ctngexts, ctngext)
		}
		return ctngexts
	}
}

func GetSequenceNumberfromCert(cert *x509.Certificate) int {
	// parse cert.CRLDistributionPoints[0] to get the sequence number
	var ctngext SequenceNumber
	// convert the string to bytes
	ctngextbytes := []byte(cert.CRLDistributionPoints[0])
	// decode the bytes to ctngext
	err := json.Unmarshal(ctngextbytes, &ctngext)
	if err != nil {
		fmt.Println("Error in GetCTngExtensions: ", err)
	}
	return ctngext.RID
}

func GetLoggerInfofromCert(cert *x509.Certificate) []CTngExtension {
	var ctngexts []CTngExtension
	if len(cert.CRLDistributionPoints) == 1 {
		return nil
	} else {
		for _, ext := range cert.CRLDistributionPoints[1:] {
			var ctngext CTngExtension
			// convert the string to bytes
			ctngextbytes := []byte(ext)
			// decode the bytes to ctngext
			err := json.Unmarshal(ctngextbytes, &ctngext)
			if err != nil {
				fmt.Println("Error in GetCTngExtensions: ", err)
			}
			ctngexts = append(ctngexts, ctngext)
		}
		return ctngexts
	}
}

func GetCTngExtensionCount(cert *x509.Certificate) int {
	return len(cert.CRLDistributionPoints)
}

func GetPrecertfromCert(cert *x509.Certificate) *x509.Certificate {
	// only keep the first ctng extension in CRLDistributionPoints
	var ctngext CTngExtension
	fmt.Sscanf(cert.CRLDistributionPoints[0], "%v", &ctngext)
	cert.CRLDistributionPoints = []string{fmt.Sprintf("%v", ctngext)}
	return cert
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
		SignaturePublicMap: crypto.RSAPublicMap{},
		RSAPrivateKey:      rsa.PrivateKey{},
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
	path := ctx.StoragePath
	data := [][]any{}
	//fmt.Println(path)
	certs := ctx.CurrentCertificatePool.GetCerts()
	// iterate through all certs
	for _, cert := range certs {
		ctngexts := GetCTngExtensions(&cert)
		data = append(data, ctngexts)
	}
	//write to file
	util.CreateFile(path)
	util.WriteData(path, data)
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
		PublicKey:              cryptoconfig.SignaturePublicMap[cryptoconfig.SelfID],
		PrivateKey:             cryptoconfig.RSAPrivateKey,
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
