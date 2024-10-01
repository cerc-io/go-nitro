<h1 align="center">
<div><img src="https://statechannels.org/favicon.ico"><br>
go-nitro
</h1>

<p align="center">Implementation of the <a href="https://docs.statechannels.org">Nitro State Channels Framework</a> in Golang and Solidity.</p>

`go-nitro` is an implementation of a node in a nitro state channel network. It is software that:

- manages a secret "channel" key
- crafts blockchain transactions (to allow the user to join and exit the network)
- crafts, signs, and sends state channel updates to counterparties in the network
- listens to blockchain events
- listens for counterparty messages
- stores important data to allow for recovery from counterparty inactivity / malice
- understands how to perform these functions safely, without risking any funds

## Usage

> ⚠️ Go-nitro is pre-production software ⚠️

Go-nitro can be consumed either as [library code](./node/readme.md) or run as an [independent process](./doc.go) and interfaced with remote procedure calls (recommended).

## Contributing

Please see [contributing.md](./contributing.md)

## ADRs

Architectural decision records may be viewed [here](./.adr/0000-adrs.md).

## Testing

> Pre-requisite: [generate a TLS certificate](./tls/readme.md)

Run the tests from repo root:

```
go test ./... -count=2 -shuffle=on -timeout 1m -v -failfast
```

## On-chain code

The on-chain component of Nitro (i.e. the solidity contracts) are housed in the [`nitro-protocol`](./packages/nitro-protocol/readme.md) directory. This directory contains an yarn workspace with a hardhat / typechain / jest toolchain.

## Demo

### Setup

- Follow this [doc](https://book.getfoundry.sh/getting-started/installation) to set up foundry to run anvil chain

  - Use an older foundry version to work with go-nitro

    ```bash
    foundryup --version nightly-cafc2606a2187a42b236df4aa65f4e8cdfcea970
    ```

- Start anvil chain:

  ```bash
  anvil  --chain-id 1337 --block-time 1 --port 8545
  ```

- Install dependencies

  ```bash
  # Install Node.js dependencies and build the repo
  yarn && yarn build

  # Install go dependencies
  go mod tidy && go build
  ```

- Deploy the Nitro protocol contracts:

  - Change directory to `nitro-protocol`

    ```bash
    cd packages/nitro-protocol
    ```

  - Deploy nitro contracts and custom token

    ```bash
    # Set environment variables
    export GETH_URL="http://127.0.0.1:8545"
    export GETH_CHAIN_ID=1337
    export GETH_DEPLOYER_PK=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80
    export DISABLE_DETERMINISTIC_DEPLOYMENT=true

    # variables for token name, token symbol and initial supply
    export TOKEN_NAME="TestToken1"
    export TOKEN_SYMBOL="TT"

    # Note: Token supply denotes actual number of tokens and not the supply in Wei
    export INITIAL_TOKEN_SUPPLY="10000000"

    # Deploy contracts
    yarn contracts:deploy-geth

    # Deploy custom token 1
    # Note the address of custom token 1
    yarn contracts:deploy-token-geth

    # Deploy custom token 2
    # Note the address of custom token 2
    export TOKEN_NAME="TestToken2"
    yarn contracts:deploy-token-geth
    ```

    Note: On restarting the chain, make sure to remove `packages/nitro-protocol/hardhat-deployments` when redeploying contracts

    ```bash
    rm -rf hardhat-deployments
    ```

  - Send custom tokens to Bob

    ```bash
    # Export variables for token addresses and Bob address
    export ASSET_ADDRESS_1="<Custom token 1 Address>"
    export ASSET_ADDRESS_2="<Custom token 2 Address>"
    export B_CHAIN_ADDRESS="0x70997970C51812dc3A010C7d01b50e0d17dc79C8"

    # Send tokens to Bob
    yarn hardhat transfer --contract $ASSET_ADDRESS_1 --to $B_CHAIN_ADDRESS --amount 1000 --network geth
    yarn hardhat transfer --contract $ASSET_ADDRESS_2 --to $B_CHAIN_ADDRESS --amount 1000 --network geth
    ```

  - Change directory to root directory

    ```bash
    cd ../../
    ```

- Generate TLS certificate

  - Change directory to `tls`

    ```bash
    cd tls
    ```

  - Follow [README](tls/readme.md)

  - Change directory to root directory

    ```bash
    cd ../
    ```

- Install nitro-rpc-client package globally:

  ```bash
  # In go-nitro
  npm install -g ./packages/nitro-rpc-client

  # Confirm global installation by running
  nitro-rpc-client --version
  ```

- Run go-nitro node for Alice in new terminal:

  - Create node config for Alice

    ```bash
    cat <<EOF > cmd/test-configs/alice.toml
    usedurablestore = true
    msgport = 3006
    rpcport = 4006
    pk = "2d999770f7b5d49b694080f987b82bbc9fc9ac2b4dcc10b0f8aba7d700f69c6d"
    chainpk = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
    chainurl = "ws://127.0.0.1:8545"
    EOF
    ```

  - Start node for Alice

    ```
    source ./packages/nitro-protocol/hardhat-deployments/geth/.contracts.env

    ./go-nitro -config cmd/test-configs/alice.toml -naaddress $NA_ADDRESS -vpaaddress $VPA_ADDRESS -caaddress $CA_ADDRESS
    ```

- Run go-nitro node for Bob in new terminal:

  - Create node config for Bob

    ```bash
    cat <<EOF > cmd/test-configs/bob.toml
    usedurablestore = true
    msgport = 3007
    rpcport = 4007
    pk = "0279651921cd800ac560c21ceea27aab0107b67daf436cdd25ce84cad30159b4"
    chainpk = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
    chainurl = "ws://127.0.0.1:8545"
    bootpeers = "/ip4/127.0.0.1/tcp/3006/p2p/16Uiu2HAmSjXJqsyBJgcBUU2HQmykxGseafSatbpq5471XmuaUqyv"
    EOF
    ```

  - Start node for Bob

    ```
    source ./packages/nitro-protocol/hardhat-deployments/geth/.contracts.env

    ./go-nitro -config cmd/test-configs/bob.toml -naaddress $NA_ADDRESS -vpaaddress $VPA_ADDRESS -caaddress $CA_ADDRESS
    ```

### Steps to demo payment between two go-nitro nodes

- In a new terminal change directory to `packages/nitro-rpc-client`

  ```bash
  cd packages/nitro-rpc-client
  ```

- Set `NODE_EXTRA_CA_CERTS` environment variable to the path of a root certificate file (rootCA.pem) generated by mkcert

  ```bash
  export NODE_EXTRA_CA_CERTS="$(mkcert -CAROOT)/rootCA.pem"
  ```

- Create a ledger channel:

  ```bash
  npm exec -c 'nitro-rpc-client direct-fund 0xBBB676f9cFF8D242e9eaC39D063848807d3D1D94 -p 4006'
  ```

  Example output

  ```bash
  Objective started DirectFunding-0x16e30bfaa0a3ebcf1347ddcdd42df29dc960c75d0d3de530fb69ec0cbeebd8fa
  Channel Open 0x16e30bfaa0a3ebcf1347ddcdd42df29dc960c75d0d3de530fb69ec0cbeebd8fa
  ```

- Assign ledger channel id in output log above (`Channel Open <LEDGER_CHANNEL_ID>`) to an environment variable

  ```bash
  export LEDGER_CHANNEL_ID=<LEDGER_CHANNEL_ID>
  ```

- Check ledger channel info:

  ```bash
  npm exec -c 'nitro-rpc-client get-ledger-channel $LEDGER_CHANNEL_ID -p 4006'
  ```

  Example output

  ```bash
  {
    ID: '0xf6523e28b39de1e9afa65e2d29c23e22949d4d4ed55137cd208d035d4a88467f',
    Status: 'Open',
    Balance: {
      AssetAddress: '0x0000000000000000000000000000000000000000',
      Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
      Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
      MyBalance: 1000000n,
      TheirBalance: 1000000n
    }
  }
  ```

- Create a virtual payment channel:

  ```bash
  npm exec -c 'nitro-rpc-client virtual-fund 0xBBB676f9cFF8D242e9eaC39D063848807d3D1D94 -p 4006'
  ```

  Example output

  ```bash
  Objective started VirtualFund-0x25676acc207865bd16c12eb4507784c0d1d3997945325a37e131d985a879bdab
  Channel Open 0x25676acc207865bd16c12eb4507784c0d1d3997945325a37e131d985a879bdab
  ```

- Assign payment channel id in output log above (`Channel Open <PAYMENT_CHANNEL_ID>`) to an environment variable

  ```bash
  export PAYMENT_CHANNEL_ID=<PAYMENT_CHANNEL_ID>
  ```

- Check ledger channel info:

  ```bash
  npm exec -c 'nitro-rpc-client get-ledger-channel $LEDGER_CHANNEL_ID -p 4006'
  ```

  Example output

  ```bash
  {
    ID: '0xdf27ffafa9fbdd5f06821a755a08982adfcdc8ea7bd12638b07c279672faf8b6',
    Status: 'Open',
    Balance: {
      AssetAddress: '0x0000000000000000000000000000000000000000',
      Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
      Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
      MyBalance: 999000n,
      TheirBalance: 999000n
    }
  }
  ```

- Check virtual channel info:

  ```bash
  npm exec -c 'nitro-rpc-client get-payment-channel $PAYMENT_CHANNEL_ID -p 4006'
  ```

  Example output

  ```bash
  {
    ID: '0xc024a21a6c2626b100b9f4571e788f6c4ffedbd03ab7a2031d2db6929b375d4e',
    Status: 'Open',
    Balance: {
      AssetAddress: '0x0000000000000000000000000000000000000000',
      Payee: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
      Payer: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
      PaidSoFar: 0n,
      RemainingFunds: 1000n
    }
  }
  ```

- Make payment from Alice to Bob:

  ```bash
  npm exec -c 'nitro-rpc-client pay $PAYMENT_CHANNEL_ID 50 -p 4006'
  ```

  Example output

  ```bash
  {
    Amount: 50,
    Channel: '0xc024a21a6c2626b100b9f4571e788f6c4ffedbd03ab7a2031d2db6929b375d4e'
  }
  ```

- Check virtual channel info:

  ```bash
  npm exec -c 'nitro-rpc-client get-payment-channel $PAYMENT_CHANNEL_ID -p 4006'
  ```

  Example output

  ```bash
  {
    ID: '0xc024a21a6c2626b100b9f4571e788f6c4ffedbd03ab7a2031d2db6929b375d4e',
    Status: 'Open',
    Balance: {
      AssetAddress: '0x0000000000000000000000000000000000000000',
      Payee: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
      Payer: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
      PaidSoFar: 50n,
      RemainingFunds: 950n
    }
  }
  ```

- Close the virtual payment channel:

  ```bash
  npm exec -c 'nitro-rpc-client virtual-defund $PAYMENT_CHANNEL_ID -p 4006'
  ```

  Example output

  ```bash
  Objective started VirtualDefund-0xc024a21a6c2626b100b9f4571e788f6c4ffedbd03ab7a2031d2db6929b375d4e
  Channel complete 0xc024a21a6c2626b100b9f4571e788f6c4ffedbd03ab7a2031d2db6929b375d4e
  ```

- Check virtual channel info:

  ```bash
  npm exec -c 'nitro-rpc-client get-payment-channel $PAYMENT_CHANNEL_ID -p 4006'
  ```

  Example output

  ```bash
  {
    ID: '0xc024a21a6c2626b100b9f4571e788f6c4ffedbd03ab7a2031d2db6929b375d4e',
    Status: 'Complete',
    Balance: {
      AssetAddress: '0x0000000000000000000000000000000000000000',
      Payee: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
      Payer: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
      PaidSoFar: 50n,
      RemainingFunds: 950n
    }
  }
  ```

- Check ledger channel info:

  ```bash
  npm exec -c 'nitro-rpc-client get-ledger-channel $LEDGER_CHANNEL_ID -p 4006'
  ```

  Example output

  ```bash
  {
    ID: '0xdf27ffafa9fbdd5f06821a755a08982adfcdc8ea7bd12638b07c279672faf8b6',
    Status: 'Open',
    Balance: {
      AssetAddress: '0x0000000000000000000000000000000000000000',
      Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
      Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
      MyBalance: 999950n,
      TheirBalance: 1000050n
    }
  }
  ```

- Close the ledger channel:

  ```bash
  npm exec -c 'nitro-rpc-client direct-defund $LEDGER_CHANNEL_ID -p 4006'
  ```

- Check ledger channel info:

  ```bash
  npm exec -c 'nitro-rpc-client get-ledger-channel $LEDGER_CHANNEL_ID -p 4006'
  ```

  Example output

  ```bash
  {
    ID: '0xf6523e28b39de1e9afa65e2d29c23e22949d4d4ed55137cd208d035d4a88467f',
    Status: 'Complete',
    Balance: {
      AssetAddress: '0x0000000000000000000000000000000000000000',
      Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
      Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
      MyBalance: 999950n,
      TheirBalance: 1000050n
    }
  }
  ```

- Check on chain balance for Alice

  ```bash
  echo $(
    printf "Result: %d" $(
      curl -sk -X POST -H "Content-Type: application/json" --data '{
        "jsonrpc":"2.0",
        "method":"eth_getBalance",
        "params": ["0xAAA6628Ec44A8a742987EF3A114dDFE2D4F7aDCE", "latest"],
        "id":1
      }' http://localhost:8545 | jq -r '.result'
    )
  )
  ```

  Example output

  ```bash
  Result: 999950
  ```

- Check on chain balance for Bob

  ```bash
  echo $(
    printf "Result: %d" $(
      curl -sk -X POST -H "Content-Type: application/json" --data '{
        "jsonrpc":"2.0",
        "method":"eth_getBalance",
        "params": ["0xBBB676f9cFF8D242e9eaC39D063848807d3D1D94", "latest"],
        "id":1
      }' http://localhost:8545 | jq -r '.result'
    )
  )
  ```

  Example output

  ```bash
  Result: 1000050
  ```

### Steps to demo swap between two go-nitro nodes

- In a new terminal, set `NODE_EXTRA_CA_CERTS` environment variable to the path of a root certificate file (rootCA.pem) generated by mkcert

  ```bash
  export NODE_EXTRA_CA_CERTS="$(mkcert -CAROOT)/rootCA.pem"
  ```

- Set environment variables

  ```bash
  export BOB_ADDRESS="0xBBB676f9cFF8D242e9eaC39D063848807d3D1D94"
  export ASSET_ADDRESS_1=<deployed custom token address 1>
  export ASSET_ADDRESS_2=<deployed custom token address 2>

  ```

- Create a multi assets ledger channel

  ```bash
  nitro-rpc-client direct-fund $BOB_ADDRESS --asset "$ASSET_ADDRESS_1:500,500" --asset "$ASSET_ADDRESS_2:500,500" -p 4006

  export LEDGER_CHANNEL_ID=<ledger-channel-id>
  ```

  Example ouput

  ```bash
  Objective started DirectFunding-0xc7b652e6c0a5e2c1c691597397d44fc0d40a73297f9997062299b102cc8d4e96
  Channel Open 0xc7b652e6c0a5e2c1c691597397d44fc0d40a73297f9997062299b102cc8d4e96
  ```

- Check ledger channel info

  ```bash
  nitro-rpc-client get-ledger-channel $LEDGER_CHANNEL_ID -p 4006
  ```

  Example output

  ```bash
  {
    ID: '0xc7b652e6c0a5e2c1c691597397d44fc0d40a73297f9997062299b102cc8d4e96',
    Status: 'Open',
    Balances: [
      {
        AssetAddress: '0xcf7ed3acca5a467e9e704c703e8d87f634fb0fc9',
        Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
        Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
        MyBalance: 500n,
        TheirBalance: 500n
      },
      {
        AssetAddress: '0xdc64a140aa3e981100a9beca4e685f962f0cf6c9',
        Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
        Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
        MyBalance: 500n,
        TheirBalance: 500n
      }
    ],
    ChannelMode: 'Open'
  }
  ```

- Create a multi assets swap channel

  ```bash
  nitro-rpc-client swap-fund $BOB_ADDRESS --asset "$ASSET_ADDRESS_1:200,200" --asset "$ASSET_ADDRESS_2:100,100" -p 4006

  export SWAP_CHANNEL_ID=<swap-channel-id>
  ```

  Example output

  ```bash
  Objective started SwapFund-0x9e1950864b8c704411a6dd790008302c3d5a875a544235cc5f423682d012adc1
  Channel open 0x9e1950864b8c704411a6dd790008302c3d5a875a544235cc5f423682d012adc1
  ```

- Check swap channel info

  ```bash
  nitro-rpc-client get-swap-channel $SWAP_CHANNEL_ID -p 4006
  ```

  Example output

  ```bash
  {
   ID: '0x9e1950864b8c704411a6dd790008302c3d5a875a544235cc5f423682d012adc1',
   Status: 'Open',
   Balances: [
     {
       AssetAddress: '0xcf7ed3acca5a467e9e704c703e8d87f634fb0fc9',
       Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
       Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
       MyBalance: 200n,
       TheirBalance: 200n
     },
     {
       AssetAddress: '0xdc64a140aa3e981100a9beca4e685f962f0cf6c9',
       Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
       Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
       MyBalance: 100n,
       TheirBalance: 100n
     }
   ]
  }
  ```

- Conduct swap through swap channel

  ```bash
  nitro-rpc-client swap $SWAP_CHANNEL_ID  --AssetIn "$ASSET_ADDRESS_1:20" --AssetOut "$ASSET_ADDRESS_2:10" -p 4006
  ```

  Example ouput

  ```bash
  {
   SwapAssetsData: {
     TokenIn: '0xcf7ed3acca5a467e9e704c703e8d87f634fb0fc9',
     TokenOut: '0xdc64a140aa3e981100a9beca4e685f962f0cf6c9',
     AmountIn: 20,
     AmountOut: 10
   },
   Channel: '0x9e1950864b8c704411a6dd790008302c3d5a875a544235cc5f423682d012adc1'
  }
  ```

- Check pending swap awaiting confirmation for this swap channel

  ```bash
  nitro-rpc-client get-pending-swap $SWAP_CHANNEL_ID  -p 4007
  ```

  Example ouput

  ```bash
  {
   Id: '0xb9e809059a92be1c22339d1e6a6d58b908f4dbd0006c0722793b2eec21475614',
   ChannelId: '0x9e1950864b8c704411a6dd790008302c3d5a875a544235cc5f423682d012adc1',
   Exchange: {
     TokenIn: '0xcf7ed3acca5a467e9e704c703e8d87f634fb0fc9',
     TokenOut: '0xdc64a140aa3e981100a9beca4e685f962f0cf6c9',
     AmountIn: 20,
     AmountOut: 10
   },
   Sigs: {
     '0': '0x8cfa6c7c8aec9089fc57b1fe649d64dcd8205905f92b1f6b6b104f64e8967e285d1d751620ef2757287993b6347237e26ab315d31ab98feaf98cd71022d0e4321c'
   },
   Nonce: 736609862712516500
  }
  ```

  - Set environment variable for swap Id using Id field from the output above

    ```bash
    export SWAP_ID=<swap id>
    ```

- Bob decides to accept / reject the incoming swap

  ```bash
  # To accept incoming swap
  nitro-rpc-client confirm-swap $SWAP_ID accepted -p 4007

  # Example output
  # Confirming Swap with accepted

  # To reject incoming swap
  nitro-rpc-client confirm-swap $SWAP_ID rejected -p 4007

  # Example output
  # Confirming Swap with rejected
  ```

- Check swap channel info

  ```bash
  nitro-rpc-client get-swap-channel $SWAP_CHANNEL_ID -p 4007
  ```

  Example output

  ```bash
  {
   ID: '0x9e1950864b8c704411a6dd790008302c3d5a875a544235cc5f423682d012adc1',
   Status: 'Open',
   Balances: [
     {
       AssetAddress: '0xcf7ed3acca5a467e9e704c703e8d87f634fb0fc9',
       Me: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
       Them: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
       MyBalance: 220n,
       TheirBalance: 180n
     },
     {
       AssetAddress: '0xdc64a140aa3e981100a9beca4e685f962f0cf6c9',
       Me: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
       Them: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
       MyBalance: 90n,
       TheirBalance: 110n
     }
   ]
  }
  ```

- Defund the swap channel

  ```bash
  nitro-rpc-client swap-defund $SWAP_CHANNEL_ID -p 4006
  ```

  Example output

  ```bash
  Objective started SwapDefund-0x9e1950864b8c704411a6dd790008302c3d5a875a544235cc5f423682d012adc1
  Objective complete SwapDefund-0x9e1950864b8c704411a6dd790008302c3d5a875a544235cc5f423682d012adc1
  ```

- Defund the ledger channel

  ```bash
  nitro-rpc-client direct-defund $LEDGER_CHANNEL_ID -p 4006
  ```

  Example output

  ```bash
  Objective started DirectDefunding-0xc7b652e6c0a5e2c1c691597397d44fc0d40a73297f9997062299b102cc8d4e96
  Objective Complete 0xc7b652e6c0a5e2c1c691597397d44fc0d40a73297f9997062299b102cc8d4e96
  ```

- Check ledger channel info

  ```bash
  nitro-rpc-client get-ledger-channel $LEDGER_CHANNEL_ID -p 4006
  ```

  Example output

  ```bash
  {
   ID: '0xc7b652e6c0a5e2c1c691597397d44fc0d40a73297f9997062299b102cc8d4e96',
   Status: 'Complete',
   Balances: [
     {
       AssetAddress: '0xcf7ed3acca5a467e9e704c703e8d87f634fb0fc9',
       Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
       Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
       MyBalance: 480n,
       TheirBalance: 520n
     },
     {
       AssetAddress: '0xdc64a140aa3e981100a9beca4e685f962f0cf6c9',
       Me: '0xaaa6628ec44a8a742987ef3a114ddfe2d4f7adce',
       Them: '0xbbb676f9cff8d242e9eac39d063848807d3d1d94',
       MyBalance: 510n,
       TheirBalance: 490n
     }
   ],
   ChannelMode: 'Open'
  }
  ```

## Steps to retry dropped txs

### Direct fund

- Check status of direct-fund objective

  ```bash
  nitro-rpc-client get-objective <Objective ID> -p <RPC port of the L1 node>

  # Expected output:
  # {
  #  "Status": 1,
  #  "DroppedTx": {
  #    "DroppedTxHash": "0x339079c724690c6b99e15b5f04fc4e36ccdab30a83602734057c4e2f6fd04afe",
  #    "ChannelId": "0xb08b6c111e9d0b0cb0ad31274132ac7e7542242e2a80201d9e9e5a2e78fe197d",
  #    "EventName": "Deposited"
  #  }
  # }
  ```

  - This will give you status of the objective, with dropped tx details if any

  - In the dropped tx details, you should see the trasaction that was dropped along with its `hash`, `channel ID`, and the `Event` associated with that tx

- You can retry the dropped tx using

  ```bash
  nitro-rpc-client retry-objective-tx <Objective ID> -p <RPC port of L1 node>

  # Expected output:
  # Transaction retried for objective DirectFunding-0x1df710eba9b90784ba96b6add8c574b4511c67ab4e4c29667bbac06385b965a6
  ```

### Bridged fund

- To check status of bridged-fund objective, you either need objective Id of corresponding direct-fund objective (L1 objective) or objective ID of bridged-fund objective (L2 objective)

  - Using corresponding direct-fund objective (L1 objective)

    ```bash
    nitro-rpc-client get-l2-objective-from-l1 <L1 Objective ID> -p <RPC port of the bridge>

    # Expected output:
    # {
    #  "Status": 1,
    #  "DroppedTx": {
    #    "DroppedTxHash": "0x339079c724690c6b99e15b5f04fc4e36ccdab30a83602734057c4e2f6fd04afe",
    #    "ChannelId": "0xb08b6c111e9d0b0cb0ad31274132ac7e7542242e2a80201d9e9e5a2e78fe197d",
    #    "EventName": "Deposited"
    #  }
    # }
    ```

  - Using objective ID of bridged-fund

    ```bash
    nitro-rpc-client get-objective <Objective ID> --l2 true -p <RPC port of the bridge>

    # Expected output:
    # {
    #  "Status": 1,
    #  "DroppedTx": {
    #    "DroppedTxHash": "0x339079c724690c6b99e15b5f04fc4e36ccdab30a83602734057c4e2f6fd04afe",
    #    "ChannelId": "0x4af740df92a57656c0d15f053e8413ded279411aec5793d8243cb1205de2d996",
    #    "EventName": "StatusUpdated"
    #  }
    # }
    ```

  - This will give you status of the objective, with dropped tx details if any

- You can retry the dropped tx using

  ```bash
  nitro-rpc-client retry-objective-tx <Objective ID> -p <RPC port of the L2 node / bridge>

  # Expected output:
  # Transaction retried for objective BridgedFunding-0xbda3cff692390dcc764fa9e27ddd562d9aa929447e89f5d0b4aeb780a1aea0c6
  ```

### Non-objective bridge txs

- Some txs that are performed by bridge are not part of any objective

- These txs are stored against channel ID

- To see if these txs are confirmed or not, we can query bridge to get the pending txs

  ```bash
  nitro-rpc-client get-pending-bridge-txs <Channel ID> -p <RPC port of the bridge>

  # Expected output:
  # {
  #  "tx_hash": "0x9aebbd42f3044295411e3631fcb6aa834ed5373a6d3bf368bfa09e5b74f4f6d1",
  #  "num_of_retries": 3,
  #  "is_retry_limit_reached": true,
  #  "is_l2": false
  # }
  ```

- Txs are stored against both L1 and L2 channel IDs so please check both channels see if any txs are pending

- If these txs fail, bridge retries them until retry tx threshold has been met

  - If there are any txs with `is_retry_limit_reached: false`, that means they are not auto retried by bridge just yet

- If there are any txs with `is_retry_limit_reached: true`, it means these txs failed even after auto retry

- The `get-pending-bridge-txs` command will output pending txs details along with their tx hash

- To manually retry txs that failed after auto retry, use

  ```bash
  nitro-rpc-client retry-tx <Tx hash of the failed tx> -p <RPC port of the bridge>

  # Expected output:
  # Transaction with hash 0x339079c724690c6b99e15b5f04fc4e36ccdab30a83602734057c4e2f6fd04afe retried
  ```

## License

Dual-licensed under [MIT](https://opensource.org/licenses/MIT) + [Apache 2.0](http://www.apache.org/licenses/LICENSE-2.0)
