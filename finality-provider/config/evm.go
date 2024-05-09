package config

const (
	defaultEVMRPCAddr = "http://127.0.0.1:8545"
)

type EVMConfig struct {
	RPCAddr string `long:"rpc-address" description:"address of the rpc server to connect to"`
}

func DefaultEVMConfig() EVMConfig {
	return EVMConfig{
		RPCAddr: defaultEVMRPCAddr,
	}
}
