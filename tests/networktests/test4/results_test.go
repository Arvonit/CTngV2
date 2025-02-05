package main

import (
	"CTngV2/gossiper"
	"CTngV2/util"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"testing"
)

func TestResults(t *testing.T) {
	// read from /gossiper_testdata/$storage_ID$/gossiper_testdata.json
	var gossiper_log_database [][]gossiper.Gossiper_log_entry
	for i := 1; i <= 4; i++ {
		var gossiper_log_map_1 gossiper.Gossiper_log
		bytedata, _ := util.ReadByte("gossiper_testdata/" + strconv.Itoa(i) + "/gossiper_testdata.json")
		json.Unmarshal(bytedata, &gossiper_log_map_1)
		//iterate through the gossiper_log_map_1, add to a list
		var gossiper_log_map_1_list []gossiper.Gossiper_log_entry
		for _, v := range gossiper_log_map_1 {
			gossiper_log_map_1_list = append(gossiper_log_map_1_list, v)
			// sort the list by GossiperLogEntry.Period
			sort.Slice(gossiper_log_map_1_list, func(i, j int) bool {
				return gossiper_log_map_1_list[i].Period < gossiper_log_map_1_list[j].Period
			})
		}
		gossiper_log_database = append(gossiper_log_database, gossiper_log_map_1_list)
	}

	for i, gossiperLog := range gossiper_log_database {
		fmt.Println("Beginning testing gossiper", i+1)

		for j, gossiperLogEntry := range gossiperLog {
			// Accusation for even periods
			if j%2 == 0 {
				if gossiperLogEntry.NUM_ACC_FULL != 1 && gossiperLogEntry.NUM_ACC_INIT != 1 {
					t.Fail()
				}
			} else {
				if gossiperLogEntry.NUM_ACC_FULL != 0 || gossiperLogEntry.NUM_ACC_INIT != 0 {
					t.Fail()
				}
			}
		}
	}
}
