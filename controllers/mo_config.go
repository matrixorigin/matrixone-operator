package controllers

type MatrixoneConfig struct {
	AddrRaft            string             `json:"addr-raft,omitempty" toml:"addr-raft,omitempty"`
	AddrAdvertiseRaft   string             `json:"addr-advertise-raft,omitempty" toml:"addr-advertise-raft,omitempty"`
	AddrClient          string             `json:"addr-client,omitempty" toml:"addr-client,omitempty"`
	AddrAdvertiseClient string             `json:"addr-advertise-client,omitempty" toml:"addr-advertise-client,omitempty"`
	DirData             string             `json:"dir-data,omitempty" toml:"dir-data,omitempty"`
	Replication         Replica            `json:"replication" toml:"replication"`
	Raft                Raft               `json:"raft" toml:"raft"`
	Prophet             Prophet            `json:"prophet" toml:"prophet"`
	ProphetEmbedEtcd    ProphetEmbedEtcd   `json:"prophet.embed-etcd" toml:"prophet.embed-etcd"`
	ProphetSchedule     ProphetSchedule    `json:"prophet.schedule" toml:"prophet.schedule"`
	ProphetReplication  ProphetReplication `json:"prophet.replication" toml:"prophet.replication"`
	Metric              Metric             `json:"metric" toml:"metric"`
}

type Replica struct {
	ShardCapacityBytes string `json:"shard-capacity-bytes" toml:"max-entry-bytes"`
}

type Raft struct {
	MaxEntryBytes string `json:"max-entry-bytes" toml:"max-entry-bytes"`
}

type Prophet struct {
	Name             string `json:"name" toml:"name"`
	RPCAddr          string `json:"rpc-addr" toml:"rpc-addr"`
	RPCAdvertiseAddr string `json:"rpc-advertise-addr" toml:"rpc-advertise-addr"`
	StorageNode      bool   `json:"storage-node" toml:"storage-node"`
}

type ProphetEmbedEtcd struct {
	Join                string `json:"join" toml:"join"`
	ClientURLs          string `json:"client-urls" toml:"client-urls"`
	AdvertiseClientURLs string `json:"advertise-client-urls" toml:"advertise-client-urls"`
	PeerURLs            string `json:"peer-urls" toml:"peer-urls"`
	AdvertisePeerURLs   string `json:"advertise-peer-urls" toml:"advertise-peer-urls"`
}

type ProphetSchedule struct {
	LowSpaceRatio   string `json:"low-space-ratio" toml:"low-space-ratio"`
	HighSpaceRation string `json:"high-space-ratio" toml:"high-space-ratio"`
}

type ProphetReplication struct {
	MaxReplicas int `json:"max-replicas" toml:"max-replicas"`
}

type Metric struct {
	Addr     string `json:"addr" toml:"addr"`
	Interval int    `json:"inteval" toml:"interval"`
	Job      string `json:"job" toml:"job"`
	Instance string `json:"instance" toml:"instance"`
}
