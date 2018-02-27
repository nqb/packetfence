package pfipset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"io/ioutil"

	"github.com/diegoguarnieri/go-conntrack/conntrack"
	ipset "github.com/digineo/go-ipset"
	"github.com/inverse-inc/packetfence/go/log"
)

type Info struct {
	Status string `json:"status"`
	Mac    string `json:"mac"`
	Ip     string `json:"ip"`
}

type PostOptions struct {
	Network  string          `json:"network,omitempty"`
	RoleID   string          `json:"role_id,omitempty"`
	Ip       string          `json:"ip,omitempty"`
	Port     string          `json:"port,omitempty"`
	Mac      string          `json:"mac,omitempty"`
	Type     string          `json:"type,omitempty"`
}

func handlePassthrough(res http.ResponseWriter, req *http.Request) {
	IPSET := pfIPSETFromContext(req.Context())

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var o PostOptions
	err = json.Unmarshal(body, &o)
	if err != nil {
		panic(err)
	}	
	Ip, valid_ipv4 := validate_ipv4(o.Ip)
	if !valid_ipv4 {
		handleError(res, http.StatusBadRequest)
		return
	}
	Port, valid_port := validate_port(o.Port)
	if !valid_port {
		handleError(res, http.StatusBadRequest)
		return
	}
	Local := req.URL.Query().Get("local")

	IPSET.jobs <- job{"Add", "pfsession_passthrough", Ip + "," + Port}
	if Local == "0" {
		updateClusterPassthrough(req.Context(), req.Body)
	}
	var result = map[string][]*Info{
		"result": {
			&Info{Ip: Ip, Status: "ACK"},
		},
	}

	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	}

}

func handleIsolationPassthrough(res http.ResponseWriter, req *http.Request) {
	IPSET := pfIPSETFromContext(req.Context())

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var o PostOptions
	err = json.Unmarshal(body, &o)
	if err != nil {
		panic(err)
	}	
	Ip, valid_ipv4 := validate_ipv4(o.Ip)
	if !valid_ipv4 {
		handleError(res, http.StatusBadRequest)
		return
	}
	Port, valid_port := validate_port(o.Port)
	if !valid_port {
		handleError(res, http.StatusBadRequest)
		return
	}
	Local := req.URL.Query().Get("local")

	IPSET.jobs <- job{"Add", "pfsession_isol_passthrough", Ip + "," + Port}
	if Local == "0" {
		updateClusterPassthroughIsol(req.Context(), req.Body)
	}
	var result = map[string][]*Info{
		"result": {
			&Info{Ip: Ip, Status: "ACK"},
		},
	}

	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	}

}

func handleLayer2(res http.ResponseWriter, req *http.Request) {
	IPSET := pfIPSETFromContext(req.Context())

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var o PostOptions
	err = json.Unmarshal(body, &o)
	if err != nil {
		panic(err)
	}	
	Ip, valid_ipv4 := validate_ipv4(o.Ip)
	if !valid_ipv4 {
		handleError(res, http.StatusBadRequest)
		return
	}
	Type, valid_type := validate_type(o.Type)
	if !valid_type {
		handleError(res, http.StatusBadRequest)
		return
	}
	Network, valid_network := validate_ipv4(o.Network)
	if !valid_network {
		handleError(res, http.StatusBadRequest)
		return
	}
	Roleid, valid_roleid := validate_roleid(o.RoleID)
	if !valid_roleid {
		handleError(res, http.StatusBadRequest)
		return
	}
	Mac, valid_mac := validate_mac(o.Mac)
	if !valid_mac {
		handleError(res, http.StatusBadRequest)
		return
	}
	Local := req.URL.Query().Get("local")

	// Update locally
	IPSET.IPSEThandleLayer2(req.Context(), Ip, Mac, Network, Type, Roleid)

	// Do we have to update the other members of the cluster
	if Local == "0" {
		updateClusterL2(req.Context(), req.Body)
	}
	var result = map[string][]*Info{
		"result": {
			&Info{Mac: Mac, Status: "ACK"},
		},
	}

	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	}
}

func (IPSET *pfIPSET) IPSEThandleLayer2(ctx context.Context, Ip string, Mac string, Network string, Type string, Roleid string) {
	logger := log.LoggerWContext(ctx)

	for _, v := range IPSET.ListALL {
		// Delete all entries with the new ip address
		r := ipset.Test(v.Name, Ip)
		if r == nil {
			IPSET.jobs <- job{"Del", v.Name, Ip}
			logger.Info("Removed " + Ip + " from " + v.Name)
		}
		// Delete all entries with old ip addresses
		Ips := IPSET.mac2ip(ctx, Mac, v)
		for _, i := range Ips {
			IPSET.jobs <- job{"Del", v.Name, i}
			logger.Info("Removed " + i + " from " + v.Name)
		}
	}
	// Add to the new ipset session
	IPSET.jobs <- job{"Add", "pfsession_" + Type + "_" + Network, Ip + "," + Mac}
	logger.Info("Added " + Ip + " " + Mac + " to pfsession_" + Type + "_" + Network)
	if Type == "Reg" {
		// Add to the ip ipset session
		IPSET.jobs <- job{"Add", "PF-iL2_ID" + Roleid + "_" + Network, Ip}
		logger.Info("Added " + Ip + " to PF-iL2_ID" + Roleid + "_" + Network)
	}
}

func handleMarkIpL2(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	IPSET := pfIPSETFromContext(req.Context())

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var o PostOptions
	err = json.Unmarshal(body, &o)
	if err != nil {
		panic(err)
	}	
	Ip, valid_ipv4 := validate_ipv4(o.Ip)
	if !valid_ipv4 {
		handleError(res, http.StatusBadRequest)
		return
	}
	Network, valid_network := validate_ipv4(o.Network)
	if !valid_network {
		handleError(res, http.StatusBadRequest)
		return
	}
	Roleid, valid_roleid := validate_roleid(o.RoleID)
	if !valid_roleid {
		handleError(res, http.StatusBadRequest)
		return
	}
	Local := req.URL.Query().Get("local")

	// Update locally
	IPSET.IPSEThandleMarkIpL2(ctx, Ip, Network, Roleid)

	// Do we have to update the other members of the cluster
	if Local == "0" {
		updateClusterMarkIpL3(req.Context(), req.Body)
	}
	var result = map[string][]*Info{
		"result": {
			&Info{Ip: Ip, Status: "ACK"},
		},
	}
	
	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	}
}

func (IPSET *pfIPSET) IPSEThandleMarkIpL2(ctx context.Context, Ip string, Network string, Roleid string) {
	IPSET.jobs <- job{"Add", "PF-iL2_ID" + Roleid + "_" + Network, Ip}
}

func handleMarkIpL3(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	IPSET := pfIPSETFromContext(req.Context())
  
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var o PostOptions
	err = json.Unmarshal(body, &o)
	if err != nil {
		panic(err)
	}	
	Ip, valid_ipv4 := validate_ipv4(o.Ip)
	if !valid_ipv4 {
		handleError(res, http.StatusBadRequest)
		return
	}
	Network, valid_network := validate_ipv4(o.Network)
	if !valid_network {
		handleError(res, http.StatusBadRequest)
		return
	}
	Roleid, valid_roleid := validate_roleid(o.RoleID)
	if !valid_roleid {
		handleError(res, http.StatusBadRequest)
		return
	}
	Local := req.URL.Query().Get("local")

	// Update locally
	IPSET.IPSEThandleMarkIpL3(ctx, Ip, Network, Roleid)

	// Do we have to update the other members of the cluster
	if Local == "0" {
		updateClusterMarkIpL3(req.Context(), req.Body)
	}
	var result = map[string][]*Info{
		"result": {
			&Info{Ip: Ip, Status: "ACK"},
		},
	}

	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	}
}

func (IPSET *pfIPSET) IPSEThandleMarkIpL3(ctx context.Context, Ip string, Network string, Roleid string) {
	IPSET.jobs <- job{"Add", "PF-iL3_ID" + Roleid + "_" + Network, Ip}
}

func handleLayer3(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	IPSET := pfIPSETFromContext(req.Context())

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var o PostOptions
	err = json.Unmarshal(body, &o)
	if err != nil {
		panic(err)
	}	
	Ip, valid_ipv4 := validate_ipv4(o.Ip)
	if !valid_ipv4 {
		handleError(res, http.StatusBadRequest)
		return
	}
	Type, valid_type := validate_type(o.Type)
	if !valid_type {
		handleError(res, http.StatusBadRequest)
		return
	}
	Network, valid_network := validate_ipv4(o.Network)
	if !valid_network {
		handleError(res, http.StatusBadRequest)
		return
	}
	Roleid, valid_roleid := validate_roleid(o.RoleID)
	if !valid_roleid {
		handleError(res, http.StatusBadRequest)
		return
	}
	Local := req.URL.Query().Get("local")

	// Update locally
	IPSET.IPSEThandleLayer3(ctx, Ip, Network, Type, Roleid)

	// Do we have to update the other members of the cluster
	if Local == "0" {
		updateClusterL3(req.Context(), req.Body)
	}

	var result = map[string][]*Info{
		"result": {
			&Info{Ip: Ip, Status: "ACK"},
		},
	}

	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	}
}

func (IPSET *pfIPSET) IPSEThandleLayer3(ctx context.Context, Ip string, Network string, Type string, Roleid string) {
	logger := log.LoggerWContext(ctx)

	// Delete all entries with the new ip address
	for _, v := range IPSET.ListALL {
		r := ipset.Test(v.Name, Ip)
		if r == nil {
			IPSET.jobs <- job{"Del", v.Name, Ip}
			logger.Info("Removed " + Ip + " from " + v.Name)
		}
	}
	// Add to the new ipset session
	IPSET.jobs <- job{"Add", "pfsession_" + Type + "_" + Network, Ip}
	logger.Info("Added " + Ip + " to pfsession_" + Type + "_" + Network)
	if Type == "Reg" {
		// Add to the ip ipset session
		IPSET.jobs <- job{"Add", "PF-iL3_ID" + Roleid + "_" + Network, Ip}
		logger.Info("Added " + Ip + " to PF-iL3_ID" + Roleid + "_" + Network)
	}
}

func (IPSET *pfIPSET) handleUnmarkMac(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	logger := log.LoggerWContext(ctx)

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var o PostOptions
	err = json.Unmarshal(body, &o)
	if err != nil {
		panic(err)
	}	
	Mac, valid_mac := validate_mac(o.Mac)
	if !valid_mac {
		handleError(res, http.StatusBadRequest)
		return
	}
	Local := req.URL.Query().Get("local")

	for _, v := range IPSET.ListALL {
		Ips := IPSET.mac2ip(req.Context(), Mac, v)
		for _, i := range Ips {
			IPSET.jobs <- job{"Del", v.Name, i}
			conn, _ := conntrack.New()
			conn.DeleteConnectionBySrcIp(i)
			logger.Info(fmt.Sprintf("Removed %s from %s", i, v.Name))
		}
	}
	// Do we have to update the other members of the cluster
	if Local == "0" {
		updateClusterUnmarkMac(req.Context(), req.Body)
	}
	var result = map[string][]*Info{
		"result": {
			&Info{Mac: Mac, Status: "ACK"},
		},
	}

	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	}
}

func (IPSET *pfIPSET) handleUnmarkIp(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	logger := log.LoggerWContext(ctx)

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		panic(err)
	}
	var o PostOptions
	err = json.Unmarshal(body, &o)
	if err != nil {
		panic(err)
	}
	Ip, valid_ipv4 := validate_ipv4(o.Ip)
	if !valid_ipv4 {
		handleError(res, http.StatusBadRequest)
		return
	}
	Local := req.URL.Query().Get("local")

	for _, v := range IPSET.ListALL {
		r := ipset.Test(v.Name, Ip)
		if r == nil {
			IPSET.jobs <- job{"Del", v.Name, Ip}
			conn, _ := conntrack.New()
			conn.DeleteConnectionBySrcIp(Ip)
			logger.Info(fmt.Sprintf("Removed %s from %s", Ip, v.Name))
		}
	}
	// Do we have to update the other members of the cluster
	if Local == "0" {
		updateClusterUnmarkIp(req.Context(), req.Body)
	}
	var result = map[string][]*Info{
		"result": {
			&Info{Ip: Ip, Status: "ACK"},
		},
	}

	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	}
}

func handleError(res http.ResponseWriter, code int) {
	var result = map[string][]*Info{
		"result": {
			&Info{Status: "NAK"},
		},
	}
	res.Header().Set("Content-Type", "application/json; charset=UTF-8")
	res.WriteHeader(code)
	if err := json.NewEncoder(res).Encode(result); err != nil {
		panic(err)
	} 
}
