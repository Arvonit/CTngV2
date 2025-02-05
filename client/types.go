package client

import (
	"CTngV2/crypto"
	"CTngV2/monitor"
	"CTngV2/util"
	"encoding/json"
	"log"
	"sync"

	"github.com/bits-and-blooms/bitset"
)

type ClientConfig struct {
	Monitor_URLs []string
	//This is the URL of the monitor where the client will get the information from
	Client_URL            string
	Port                  string
	MMD                   int
	MRD                   int
	STH_Storage_filepath  string
	CRV_Storage_filepath  string
	D1_Blacklist_filepath string
	D2_Blacklist_filepath string
}

type ClientContext struct {
	Config              *ClientConfig
	Crypto              *crypto.CryptoConfig
	Current_Monitor_URL string
	// the databases are shared resources and should be protected with mutex
	STH_database              map[string]string         // key = entity_ID + @ + Period, content = RootHash
	CRV_database              map[string]*bitset.BitSet // key = entity_ID, content = CRV
	D1_Blacklist_database     map[string]bool           // key = entity_ID + "@" + Period. content = bool
	D2_Blacklist_database     map[string]string         // key = entity_ID, content = Period since
	Monitor_Interity_database map[string]string         // key = Period, content = NUM_ACC_FULL + "@" + NUM_CON_FULL
	STH_DB_RWLock             *sync.RWMutex
	CRV_DB_RWLock             *sync.RWMutex
	D1_Blacklist_DB_RWLock    *sync.RWMutex
	D2_Blacklist_DB_RWLock    *sync.RWMutex
	// Don't need lock for monitor integerity DB because it is only checked once per period
	Config_filepath string
	Crypto_filepath string
	Status          string
}

func SaveSTHDatabase(ctx *ClientContext) {
	util.WriteData(ctx.Config.STH_Storage_filepath, ctx.STH_database)
}

func SaveCRVDatabase(ctx *ClientContext) {
	var crvstorage = make(map[string][]byte)
	for key, value := range ctx.CRV_database {
		crvstorage[key], _ = value.MarshalBinary()
	}
	util.WriteData(ctx.Config.CRV_Storage_filepath, crvstorage)
}

func LoadSTHDatabase(ctx *ClientContext) {
	databyte, err := util.ReadByte(ctx.Config.STH_Storage_filepath)
	if err != nil {
		ctx.STH_database = make(map[string]string)
		return
	}
	json.Unmarshal(databyte, &ctx.STH_database)
	if err != nil {
		log.Fatal(err)
	}
}

func LoadCRVDatabase(ctx *ClientContext) {
	databyte, err := util.ReadByte(ctx.Config.CRV_Storage_filepath)
	if err != nil {
		ctx.CRV_database = make(map[string]*bitset.BitSet)
		return
	}
	var crvstorage = make(map[string][]byte)
	err = json.Unmarshal(databyte, &crvstorage)
	if err != nil {
		log.Fatal(err)
	}
	for key, value := range crvstorage {
		var crv_entry bitset.BitSet
		crv_entry.UnmarshalBinary(value)
		ctx.CRV_database[key] = &crv_entry
	}
}

func LoadD1BlacklistDatabase(ctx *ClientContext) {
	databyte, err := util.ReadByte(ctx.Config.D1_Blacklist_filepath)
	if err != nil {
		ctx.D1_Blacklist_database = make(map[string]bool)
		return
	}
	err = json.Unmarshal(databyte, &ctx.D1_Blacklist_database)
	if err != nil {
		log.Fatal(err)
	}
}

func LoadD2BlacklistDatabase(ctx *ClientContext) {
	databyte, err := util.ReadByte(ctx.Config.D2_Blacklist_filepath)
	if err != nil {
		ctx.D2_Blacklist_database = make(map[string]string)
		return
	}
	err = json.Unmarshal(databyte, &ctx.D2_Blacklist_database)
	if err != nil {
		log.Fatal(err)
	}
}

func (ctx *ClientContext) LoadUpdate(filepath string) monitor.ClientUpdate {
	update_json, err := util.ReadByte(filepath)
	if err != nil {
		log.Fatal(err)
	}
	var update_m monitor.ClientUpdate
	err = json.Unmarshal(update_json, &update_m)
	if err != nil {
		log.Fatal(err)
	}
	return update_m
}

func (ctx *ClientContext) InitializeClientContext() {
	util.LoadConfiguration(ctx.Config, ctx.Config_filepath)
	CryptoConfig, err := crypto.ReadVerifyOnlyCryptoConfig(ctx.Crypto_filepath)
	ctx.Crypto = CryptoConfig
	// initialize the Locks for the databases
	ctx.STH_DB_RWLock = &sync.RWMutex{}
	ctx.CRV_DB_RWLock = &sync.RWMutex{}
	ctx.D1_Blacklist_DB_RWLock = &sync.RWMutex{}
	ctx.D2_Blacklist_DB_RWLock = &sync.RWMutex{}
	// initialize the databases
	ctx.STH_database = make(map[string]string)
	ctx.CRV_database = make(map[string]*bitset.BitSet)
	ctx.D1_Blacklist_database = make(map[string]bool)
	ctx.D2_Blacklist_database = make(map[string]string)
	ctx.Monitor_Interity_database = make(map[string]string)
	// load the databases
	if err != nil {
		log.Fatal(err)
	}
	if ctx.Status != "NEW" {
		LoadSTHDatabase(ctx)
		LoadCRVDatabase(ctx)
		LoadD1BlacklistDatabase(ctx)
		LoadD2BlacklistDatabase(ctx)
	}
}
