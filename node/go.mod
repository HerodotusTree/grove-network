module github.com/HerodotusTree/grove-network/node

go 1.27.2

replace github.com/HerodotusTree/grove-network/ledger => ../ledger

replace github.com/HerodotusTree/grove-network/vault => ../vault

require (
	github.com/HerodotusTree/grove-network/ledger v0.0.0-00010101000000-000000000000
	github.com/HerodotusTree/grove-network/vault v0.0.0-00010101000000-000000000000
)
