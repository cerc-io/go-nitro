// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package Bridge

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// ExitFormatAllocation is an auto generated low-level Go binding around an user-defined struct.
type ExitFormatAllocation struct {
	Destination    [32]byte
	Amount         *big.Int
	AllocationType uint8
	Metadata       []byte
}

// ExitFormatAssetMetadata is an auto generated low-level Go binding around an user-defined struct.
type ExitFormatAssetMetadata struct {
	AssetType uint8
	Metadata  []byte
}

// ExitFormatSingleAssetExit is an auto generated low-level Go binding around an user-defined struct.
type ExitFormatSingleAssetExit struct {
	Asset         common.Address
	AssetMetadata ExitFormatAssetMetadata
	Allocations   []ExitFormatAllocation
}

// IMultiAssetHolderReclaimArgs is an auto generated low-level Go binding around an user-defined struct.
type IMultiAssetHolderReclaimArgs struct {
	SourceChannelId       [32]byte
	FixedPart             INitroTypesFixedPart
	VariablePart          INitroTypesVariablePart
	SourceOutcomeBytes    []byte
	SourceAssetIndex      *big.Int
	IndexOfTargetInSource *big.Int
	TargetStateHash       [32]byte
	TargetOutcomeBytes    []byte
	TargetAssetIndex      *big.Int
}

// INitroTypesFixedPart is an auto generated low-level Go binding around an user-defined struct.
type INitroTypesFixedPart struct {
	Participants      []common.Address
	ChannelNonce      uint64
	AppDefinition     common.Address
	ChallengeDuration *big.Int
}

// INitroTypesVariablePart is an auto generated low-level Go binding around an user-defined struct.
type INitroTypesVariablePart struct {
	Outcome []ExitFormatSingleAssetExit
	AppData []byte
	TurnNum *big.Int
	IsFinal bool
}

// BridgeMetaData contains all meta data concerning the Bridge contract.
var BridgeMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"channelId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"asset\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"assetIndex\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"initialHoldings\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"finalHoldings\",\"type\":\"uint256\"}],\"name\":\"AllocationUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"destination\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"asset\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"destinationHoldings\",\"type\":\"uint256\"}],\"name\":\"Deposited\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"channelId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"assetIndex\",\"type\":\"uint256\"}],\"name\":\"Reclaimed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"bytes32\",\"name\":\"channelId\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"stateHash\",\"type\":\"bytes32\"}],\"name\":\"StatusUpdated\",\"type\":\"event\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"destination\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"allocationType\",\"type\":\"uint8\"},{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"}],\"internalType\":\"structExitFormat.Allocation[]\",\"name\":\"sourceAllocations\",\"type\":\"tuple[]\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"destination\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"allocationType\",\"type\":\"uint8\"},{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"}],\"internalType\":\"structExitFormat.Allocation[]\",\"name\":\"targetAllocations\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256\",\"name\":\"indexOfTargetInSource\",\"type\":\"uint256\"}],\"name\":\"compute_reclaim_effects\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"destination\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"allocationType\",\"type\":\"uint8\"},{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"}],\"internalType\":\"structExitFormat.Allocation[]\",\"name\":\"\",\"type\":\"tuple[]\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"initialHoldings\",\"type\":\"uint256\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"destination\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"allocationType\",\"type\":\"uint8\"},{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"}],\"internalType\":\"structExitFormat.Allocation[]\",\"name\":\"allocations\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256[]\",\"name\":\"indices\",\"type\":\"uint256[]\"}],\"name\":\"compute_transfer_effects_and_interactions\",\"outputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"destination\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"allocationType\",\"type\":\"uint8\"},{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"}],\"internalType\":\"structExitFormat.Allocation[]\",\"name\":\"newAllocations\",\"type\":\"tuple[]\"},{\"internalType\":\"bool\",\"name\":\"allocatesOnlyZeros\",\"type\":\"bool\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"destination\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"allocationType\",\"type\":\"uint8\"},{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"}],\"internalType\":\"structExitFormat.Allocation[]\",\"name\":\"exitAllocations\",\"type\":\"tuple[]\"},{\"internalType\":\"uint256\",\"name\":\"totalPayouts\",\"type\":\"uint256\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"asset\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"channelId\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"expectedHeld\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"deposit\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"holdings\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"sourceChannelId\",\"type\":\"bytes32\"},{\"components\":[{\"internalType\":\"address[]\",\"name\":\"participants\",\"type\":\"address[]\"},{\"internalType\":\"uint64\",\"name\":\"channelNonce\",\"type\":\"uint64\"},{\"internalType\":\"address\",\"name\":\"appDefinition\",\"type\":\"address\"},{\"internalType\":\"uint48\",\"name\":\"challengeDuration\",\"type\":\"uint48\"}],\"internalType\":\"structINitroTypes.FixedPart\",\"name\":\"fixedPart\",\"type\":\"tuple\"},{\"components\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"asset\",\"type\":\"address\"},{\"components\":[{\"internalType\":\"enumExitFormat.AssetType\",\"name\":\"assetType\",\"type\":\"uint8\"},{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"}],\"internalType\":\"structExitFormat.AssetMetadata\",\"name\":\"assetMetadata\",\"type\":\"tuple\"},{\"components\":[{\"internalType\":\"bytes32\",\"name\":\"destination\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"uint8\",\"name\":\"allocationType\",\"type\":\"uint8\"},{\"internalType\":\"bytes\",\"name\":\"metadata\",\"type\":\"bytes\"}],\"internalType\":\"structExitFormat.Allocation[]\",\"name\":\"allocations\",\"type\":\"tuple[]\"}],\"internalType\":\"structExitFormat.SingleAssetExit[]\",\"name\":\"outcome\",\"type\":\"tuple[]\"},{\"internalType\":\"bytes\",\"name\":\"appData\",\"type\":\"bytes\"},{\"internalType\":\"uint48\",\"name\":\"turnNum\",\"type\":\"uint48\"},{\"internalType\":\"bool\",\"name\":\"isFinal\",\"type\":\"bool\"}],\"internalType\":\"structINitroTypes.VariablePart\",\"name\":\"variablePart\",\"type\":\"tuple\"},{\"internalType\":\"bytes\",\"name\":\"sourceOutcomeBytes\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"sourceAssetIndex\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"indexOfTargetInSource\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"targetStateHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"targetOutcomeBytes\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"targetAssetIndex\",\"type\":\"uint256\"}],\"internalType\":\"structIMultiAssetHolder.ReclaimArgs\",\"name\":\"reclaimArgs\",\"type\":\"tuple\"}],\"name\":\"reclaim\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"name\":\"statusOf\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"assetIndex\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"fromChannelId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"outcomeBytes\",\"type\":\"bytes\"},{\"internalType\":\"bytes32\",\"name\":\"stateHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint256[]\",\"name\":\"indices\",\"type\":\"uint256[]\"}],\"name\":\"transfer\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"channelId\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32\",\"name\":\"stateHash\",\"type\":\"bytes32\"},{\"internalType\":\"bytes\",\"name\":\"outcomeBytes\",\"type\":\"bytes\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"asset\",\"type\":\"address\"}],\"name\":\"updateMirroredChannelStates\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608080604052346100595760028054336001600160a01b0319821681179092556001600160a01b03167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0600080a3612954908161005f8239f35b600080fdfe6080604052600436101561001257600080fd5b6000803560e01c806311e9f17814611358578063166e56cd146113095780632fb1d27014610f555780633033730e14610b0e578063566d54c614610a9b578063715018a614610a1b5780637837f977146109635780638da5cb5b1461092f578063b89659e3146101d9578063c7df14e2146101b05763f2fde38b1461009657600080fd5b346101ad5760206003193601126101ad576100af611711565b6100b7611768565b73ffffffffffffffffffffffffffffffffffffffff80911690811561012957600254827fffffffffffffffffffffffff0000000000000000000000000000000000000000821617600255167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e08380a380f35b60846040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602660248201527f4f776e61626c653a206e6577206f776e657220697320746865207a65726f206160448201527f64647265737300000000000000000000000000000000000000000000000000006064820152fd5b80fd5b50346101ad5760206003193601126101ad57604060209160043581528083522054604051908152f35b50346101ad5760031960208136011261092b5767ffffffffffffffff6004351161092b57610120908181600435360301126109275760405191820182811067ffffffffffffffff8211176108fa576040526004356004013582526024600435013567ffffffffffffffff81116107445760043501906080818336030112610744576040519161026783611438565b600481013567ffffffffffffffff8111610740578101366023820112156107405760048101359061029782611493565b916102a56040519384611470565b8083526024602084019160051b830101913683116108de57602401905b8282106108e2575050508352602481013567ffffffffffffffff81168103610740576103079160649160208601526102fc60448201611734565b604086015201611755565b606083015260208301918252604460043501359067ffffffffffffffff821161074857608090826004350136030112610744576040519061034782611438565b600481813501013567ffffffffffffffff8111610740573660238284600435010101121561074057600481838235010101359061038382611493565b916103916040519384611470565b80835260208301913660248360051b838860043501010101116108de5760248186600435010101925b60248360051b838860043501010101841061074c575050505082526024816004350101359067ffffffffffffffff821161074057610403606492600436918482350101016114c7565b60208401526104186044826004350101611755565b6040840152600435010135801515810361074857606082015260408301526064600435013567ffffffffffffffff81116107445761045d9060043691813501016114c7565b6060830190815260043560848101356080850190815260a482013560a086015260c482013560c0860152939060e4013567ffffffffffffffff8111610740576104ad9060043691813501016114c7565b928360e083015261010460043501356101008301526104fa825193516104f58751956104d8816125d7565b6104e88551604088015190612803565b835160208501209061250d565b6119e0565b90610504856119e0565b9173ffffffffffffffffffffffffffffffffffffffff6105248683611987565b51511694600260ff604061054a8161053c8688611987565b51015160a08a015190611987565b51015116036106e25760406105626105709284611987565b51015160a086015190611987565b51519473ffffffffffffffffffffffffffffffffffffffff610599610104600435013586611987565b515116036106845761064461061d610677946105ee887f4d3754632451ebba9812a9305e7bca17b67a17186a5cff93d2e9ae1b01e3d27b9a6105dc60209b6125d7565b60c08a0151908b81519101209061250d565b604061060f816105ff8d5188611987565b510151926101008a015190611987565b51015160a08801519161216c565b92855193604061062e8b5186611987565b5101528260408701515251604086015190612803565b9060405161066e81610660898201948a8652604083019061200f565b03601f198101835282611470565b51902091612720565b519251604051908152a280f35b60646040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601d60248201527f746172676574417373657420213d2067756172616e74656541737365740000006044820152fd5b60646040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601a60248201527f6e6f7420612067756172616e74656520616c6c6f636174696f6e0000000000006044820152fd5b8580fd5b8380fd5b8480fd5b833567ffffffffffffffff81116108da5760607fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffdc82858a600435010101360301126108da576040519061079e826113ed565b6107b2602482868b60043501010101611734565b825267ffffffffffffffff604482868b6004350101010135116108ce57604060043589018501820160448101350136037fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffdc01126108ce5760405161081581611454565b600480358a018601830160448101350160240135908110156108d657815267ffffffffffffffff60446004358b018701840181810135010135116108d25761087436602460446004358d018901860181810135019081013501016114c7565b6020820152602083015267ffffffffffffffff606482868b6004350101010135116108ce5760249260209283926108bc9036906004358d018901016064810135018701611513565b604082015281520194019390506103ba565b8b80fd5b8c80fd5b8d80fd5b8a80fd5b8880fd5b602080916108ef84611734565b8152019101906102c2565b6024847f4e487b710000000000000000000000000000000000000000000000000000000081526041600452fd5b8280fd5b5080fd5b50346101ad57806003193601126101ad57602073ffffffffffffffffffffffffffffffffffffffff60025416604051908152f35b50346101ad5760a06003193601126101ad5760043560243560443567ffffffffffffffff81116107445761099b9036906004016114c7565b6084359173ffffffffffffffffffffffffffffffffffffffff831680930361074857610a116020927f62e6bf5c61a11078212ba836ebb8494a794e81614008017cced73376a0892aa5946109ed611768565b87526001845260408720868852845260643560408820558381519101208286612720565b604051908152a280f35b50346101ad57806003193601126101ad57610a34611768565b8073ffffffffffffffffffffffffffffffffffffffff6002547fffffffffffffffffffffffff00000000000000000000000000000000000000008116600255167f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e08280a380f35b50346101ad5760606003193601126101ad5767ffffffffffffffff60043581811161092757610ace903690600401611513565b916024359182116101ad57610b0a610af684610aed3660048701611513565b6044359161216c565b60405191829160208352602083019061168c565b0390f35b50346101ad5760a06003193601126101ad57602480359190604467ffffffffffffffff600435823582811161074057610b4b9036906004016114c7565b9460649687359360843590811161092757610b6a9036906004016115e6565b93825b60018101808211610f29578651811015610c0757610b96610b8e8389611987565b519188611987565b511115610bab57610ba690611d2b565b610b6d565b89887f496e6469636573206d75737420626520736f7274656400000000000000000000896016604051937f08c379a000000000000000000000000000000000000000000000000000000000855260206004860152840152820152fd5b5050868691898597858d98610c1b826125d7565b83519360209889958683012090610c32918861250d565b610c3b906119e0565b9173ffffffffffffffffffffffffffffffffffffffff9384610c5d8486611987565b51511690818c528160019c8d998a8a52604082208683528a52604082205493610c86888a611987565b516040015190610c969186611d65565b91509b8585528d81526040852090898652526040842090815490610cb991611d58565b9055610cc5888a611987565b51604001528d6040518181019182528060408101610ce3908c61200f565b03601f1981018252610cf59082611470565b519020610d029187612720565b52878b528d604081208482528c52604090205490604051928352848c8401526040830152606082015260807f95655fb00939f9d12257c78a601be335cd6ce1ce12296e2f367918fcf25fe4e391a2610d5991611987565b5191868284511693015160405190610d70826113ed565b8482528882015260400190815289935b610d88578980f35b80518051851015610f245784610d9d91611987565b5151938a88610dad838551611987565b510151958a8a8260a01c15600014610ef957505084168986610e5157508180939495969781925af1610ddd61183b565b5015610df65790610df088949392611d2b565b93610d80565b88877f436f756c64206e6f74207472616e73666572204554480000000000000000000088601689604051947f08c379a00000000000000000000000000000000000000000000000000000000086526004860152840152820152fd5b6040517fa9059cbb00000000000000000000000000000000000000000000000000000000815273ffffffffffffffffffffffffffffffffffffffff9290921660048301526024820197909752959086906044908290885af1908115610eee578995610df092610ec1575b50611d2b565b610ee0908a3d8c11610ee7575b610ed88183611470565b810190611823565b508c610ebb565b503d610ece565b6040513d8d823e3d90fd5b83829993610f1d936040938b610df0999852528282209082528d52209182546117e7565b9055611d2b565b508980f35b88857f4e487b710000000000000000000000000000000000000000000000000000000081526011600452fd5b5060806003193601126101ad57610f6a611711565b9060248035906064938435948360a01c156112ae5773ffffffffffffffffffffffffffffffffffffffff821692838652602091600183526040872086885283526040872054916044358303611253578561109d578834036110425750507f87d4c0b5e30d6808bc8a94ba1c4d839b29d664151551a31753387ee9ef48429b949596610ff4916117e7565b9286526001815260408620908587525281604086205561103c604051928392836020909392919373ffffffffffffffffffffffffffffffffffffffff60408201951681520152565b0390a280f35b601f8491604051927f08c379a000000000000000000000000000000000000000000000000000000000845260048401528201527f496e636f7272656374206d73672e76616c756520666f72206465706f736974006044820152fd5b979190604051848101907f23b872dd000000000000000000000000000000000000000000000000000000008252338b82015230604482015284838201528281528960a082019180831067ffffffffffffffff841117611227576111479382918460405261110985611454565b8985527f5361666545524332303a206c6f772d6c6576656c2063616c6c206661696c656460c08201525190828c5af161114061183b565b908961186b565b80519085821592831561120f575b5050501561118d57507f87d4c0b5e30d6808bc8a94ba1c4d839b29d664151551a31753387ee9ef48429b9596975090610ff4916117e7565b837f6f74207375636365656400000000000000000000000000000000000000000000608492602a8c604051947f08c379a000000000000000000000000000000000000000000000000000000000865260048601528401527f5361666545524332303a204552433230206f7065726174696f6e20646964206e6044840152820152fd5b61121f9350820181019101611823565b388581611155565b8c827f4e487b710000000000000000000000000000000000000000000000000000000081526041600452fd5b60148491604051927f08c379a000000000000000000000000000000000000000000000000000000000845260048401528201527f68656c6420213d20657870656374656448656c640000000000000000000000006044820152fd5b6040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601f818501527f4465706f73697420746f2065787465726e616c2064657374696e6174696f6e006044820152fd5b50346101ad5760406003193601126101ad57604060209173ffffffffffffffffffffffffffffffffffffffff61133d611711565b16815260018352818120602435825283522054604051908152f35b50346101ad5760606003193601126101ad5767ffffffffffffffff6024358181116109275761138b903690600401611513565b916044359182116101ad576113ce6113e36113b6856113ad36600488016115e6565b90600435611d65565b9293919060405195869560808752608087019061168c565b9115156020860152848203604086015261168c565b9060608301520390f35b6060810190811067ffffffffffffffff82111761140957604052565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b6080810190811067ffffffffffffffff82111761140957604052565b6040810190811067ffffffffffffffff82111761140957604052565b90601f601f19910116810190811067ffffffffffffffff82111761140957604052565b67ffffffffffffffff81116114095760051b60200190565b67ffffffffffffffff811161140957601f01601f191660200190565b81601f8201121561150e578035906114de826114ab565b926114ec6040519485611470565b8284526020838301011161150e57816000926020809301838601378301015290565b600080fd5b9080601f8301121561150e57813561152a81611493565b9260409161153a83519586611470565b808552602093848087019260051b8401019381851161150e57858401925b858410611569575050505050505090565b67ffffffffffffffff843581811161150e57860191608080601f19858803011261150e5784519061159982611438565b8a8501358252858501358b8301526060908186013560ff8116810361150e578784015285013593841161150e576115d7878c809796819701016114c7565b90820152815201930192611558565b81601f8201121561150e578035916115fd83611493565b9261160b6040519485611470565b808452602092838086019260051b82010192831161150e578301905b828210611635575050505090565b81358152908301908301611627565b60005b8381106116575750506000910152565b8181015183820152602001611647565b90601f19601f60209361168581518092818752878088019101611644565b0116010190565b908082519081815260208091019281808460051b8301019501936000915b8483106116ba5750505050505090565b909192939495848061170183601f1986600196030187528a51805182528381015184830152604060ff81830151169083015260608091015191608080928201520190611667565b98019301930191949392906116aa565b6004359073ffffffffffffffffffffffffffffffffffffffff8216820361150e57565b359073ffffffffffffffffffffffffffffffffffffffff8216820361150e57565b359065ffffffffffff8216820361150e57565b73ffffffffffffffffffffffffffffffffffffffff60025416330361178957565b60646040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152602060248201527f4f776e61626c653a2063616c6c6572206973206e6f7420746865206f776e65726044820152fd5b919082018092116117f457565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b9081602091031261150e5751801515810361150e5790565b3d15611866573d9061184c826114ab565b9161185a6040519384611470565b82523d6000602084013e565b606090565b919290156118e6575081511561187f575090565b3b156118885790565b60646040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601d60248201527f416464726573733a2063616c6c20746f206e6f6e2d636f6e74726163740000006044820152fd5b8251909150156118f95750805190602001fd5b611937906040519182917f08c379a0000000000000000000000000000000000000000000000000000000008352602060048401526024830190611667565b0390fd5b8051156119485760200190565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b8051600110156119485760400190565b80518210156119485760209160051b010190565b81601f8201121561150e5780516119b1816114ab565b926119bf6040519485611470565b8184526020828401011161150e576119dd9160208085019101611644565b90565b8051810160208282031261150e57602082015167ffffffffffffffff811161150e5760208201603f82850101121561150e576020818401015190611a2382611493565b93611a316040519586611470565b82855260208501916020850160408560051b83850101011161150e57604081830101925b60408560051b83850101018410611a6f5750505050505090565b835167ffffffffffffffff811161150e5782840101601f1990606082828a03011261150e57604051916060830183811067ffffffffffffffff821117611c9257604052604082015173ffffffffffffffffffffffffffffffffffffffff8116810361150e578352606082015167ffffffffffffffff811161150e57604090830191828b03011261150e5760405190611b0682611454565b6040810151600481101561150e57825260608101519067ffffffffffffffff821161150e576040611b3d9260208d0192010161199b565b60208201526020830152608081015167ffffffffffffffff811161150e5760208901605f82840101121561150e576040818301015190611b7c82611493565b92611b8a6040519485611470565b828452602084019060208c0160608560051b85840101011161150e57606083820101915b60608560051b85840101018310611bd75750505050506040820152815260209384019301611a55565b825167ffffffffffffffff811161150e57608083860182018f037fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc0011261150e5760405191611c2583611438565b8386018201606081015184526080810151602085015260a0015160ff8116810361150e57604084015260c0828786010101519267ffffffffffffffff841161150e578f602094936060869586611c829401928b8a0101010161199b565b6060820152815201920191611bae565b602460007f4e487b710000000000000000000000000000000000000000000000000000000081526041600452fd5b90611cca82611493565b604090611cd982519182611470565b838152601f19611ce98295611493565b0191600091825b848110611cfe575050505050565b6020908351611d0c81611438565b8581528286818301528686830152606080830152828501015201611cf0565b7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff81146117f45760010190565b919082039182116117f457565b919290835180151560001461200457611d7d90611cc0565b91600091611d8b8151611cc0565b95600190818097938960009586935b611da8575b50505050505050565b909192939495978351851015611ffb57611dc28585611987565b5151611dce8685611987565b515260409060ff8083611de18989611987565b5101511683611df08988611987565b510152606080611e008989611987565b51015181611e0e8a89611987565b51015260209384611e1f8a8a611987565b51015186811115611ff5575085965b8d8b51908b8215928315611fcb575b505050600014611f9a5750600283828f611e57908c611987565b5101511614611f3d578f96959493868f918f611efa90611f0094611f0c988f988f908f91611f069a898f94611ecf8f869288611eaa83611ea48884611e9c848e611987565b510151611d58565b93611987565b510152611eb78187611987565b51519885611ec58389611987565b5101511695611987565b51015194825196611edf88611438565b8752860152840152820152611ef48383611987565b52611987565b506117e7565b9c611d2b565b95611987565b510151611f34575b611f2791611f2191611d58565b93611d2b565b91909493928a9085611d9a565b60009a50611f14565b8460649151907f08c379a00000000000000000000000000000000000000000000000000000000082526004820152601b60248201527f63616e6e6f74207472616e7366657220612067756172616e74656500000000006044820152fd5b9050611f0c925088915084611fb583959e989796958a611987565b51015184611fc38484611987565b510152611987565b821092509082611fe0575b50508e8b38611e3d565b611fec9192508d611987565b51148a8f611fd6565b96611e2e565b97829150611d9f565b50611d7d8151611cc0565b90815180825260208092019182818360051b85019501936000915b84831061203a5750505050505090565b9091929394958181038352865173ffffffffffffffffffffffffffffffffffffffff815116825285810151906060918288850152805160048082101561213e575091886120a09285948796839801520151604092839182608088015260a0870190611667565b91015193828183039101528351908181528581019286808460051b8401019601946000915b8483106120e857505050505050509080600192980193019301919493929061202a565b919395978061212a89601f1987600196989a9c03018b526080878d5180518452858101518685015260ff89820151168985015201519181898201520190611667565b99019701930190918b9796959394926120c5565b6021907f4e487b71000000000000000000000000000000000000000000000000000000006000525260246000fd5b80517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff81019081116117f4576121a190611cc0565b916121ac8483611987565b516060810151926040948551916121c283611454565b60009586845286602080950152878180518101031261250957878051916121e883611454565b85810151835201519084810191825287998890899c8a988b5b87518d101561236a578f848e1461235b578c8f8f9061226e858f8f908f6122288782611987565b515195826122368984611987565b510151606061224c8a60ff85611ec58389611987565b5101519382519861225c8a611438565b89528801528601526060850152611987565b52612279848d611987565b5087159081612345575b5061230b575b5015806122f6575b6122a8575b611f006122a291611d2b565b9b612201565b9e50986122eb908f6122d68b6122cc8f6122c28391611977565b510151938d611987565b51019182516117e7565b9052896122e28d611977565b510151906117e7565b60019e909990612296565b506123018d89611987565b5151875114612291565b829c9196506122e2818c6123348f6122cc61233b988261232b819961193b565b51015194611987565b905261193b565b996001948c612289565b61235091508b611987565b51518851148f612283565b509b9d506122a260019e611d2b565b509899509c969a995050939992505050156124ac571561244f57156123f2578301510361239657505090565b6064925051907f08c379a000000000000000000000000000000000000000000000000000000000825280600483015260248201527f746f74616c5265636c61696d6564213d67756172616e7465652e616d6f756e746044820152fd5b6064848451907f08c379a00000000000000000000000000000000000000000000000000000000082526004820152601460248201527f636f756c64206e6f742066696e642072696768740000000000000000000000006044820152fd5b6064858551907f08c379a00000000000000000000000000000000000000000000000000000000082526004820152601360248201527f636f756c64206e6f742066696e64206c656674000000000000000000000000006044820152fd5b6064868651907f08c379a00000000000000000000000000000000000000000000000000000000082526004820152601560248201527f636f756c64206e6f742066696e642074617267657400000000000000000000006044820152fd5b8680fd5b9161254b9060005260006020526040600020548060d01c9173ffffffffffffffffffffffffffffffffffffffff65ffffffffffff8360a01c16921690565b9391505061256f73ffffffffffffffffffffffffffffffffffffffff9283926126e6565b1691160361257957565b60646040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601560248201527f696e636f72726563742066696e6765727072696e7400000000000000000000006044820152fd5b6125e09061267f565b6003811015612650576002036125f257565b60646040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601660248201527f4368616e6e656c206e6f742066696e616c697a65642e000000000000000000006044820152fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052602160045260246000fd5b6126c365ffffffffffff9160005260006020526040600020548060d01c9173ffffffffffffffffffffffffffffffffffffffff65ffffffffffff8360a01c16921690565b5090501680156000146126d65750600090565b42106126e157600290565b600190565b73ffffffffffffffffffffffffffffffffffffffff916040519060208201928352604082015260408152612719816113ed565b5190201690565b9179ffffffffffff00000000000000000000000000000000000000007fffffffffffff00000000000000000000000000000000000000000000000000009173ffffffffffffffffffffffffffffffffffffffff6127e66127b38760005260006020526040600020548060d01c9173ffffffffffffffffffffffffffffffffffffffff65ffffffffffff8360a01c16921690565b509390968160606040516127c681611438565b65ffffffffffff808c1682528816602082015283604082015201526126e6565b1694600052600060205260a01b169160d01b161717604060002055565b60609181519060209067ffffffffffffffff82850151169273ffffffffffffffffffffffffffffffffffffffff906040958287820151169065ffffffffffff988991015116908751968688019460a089016080875285518091528860c08b019601916000905b82821061290557505050508888015260608701526080860152849003601f1981810186526128ff9590949392916128a09082611470565b519020956128e883830151916128d860608551928a8701511695015115159360a08a519a8b9889019c8d5288015260c0870190611667565b908686830301606087015261200f565b91608084015260a083015203908101835282611470565b51902090565b835181168852968a0196928a019260019091019061286956fea2646970667358221220d868d7d8ad6afd558f9544c6b601250a3124d377ea3074fd3bdab48efa82922a64736f6c63430008110033",
}

// BridgeABI is the input ABI used to generate the binding from.
// Deprecated: Use BridgeMetaData.ABI instead.
var BridgeABI = BridgeMetaData.ABI

// BridgeBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use BridgeMetaData.Bin instead.
var BridgeBin = BridgeMetaData.Bin

// DeployBridge deploys a new Ethereum contract, binding an instance of Bridge to it.
func DeployBridge(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Bridge, error) {
	parsed, err := BridgeMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(BridgeBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Bridge{BridgeCaller: BridgeCaller{contract: contract}, BridgeTransactor: BridgeTransactor{contract: contract}, BridgeFilterer: BridgeFilterer{contract: contract}}, nil
}

// Bridge is an auto generated Go binding around an Ethereum contract.
type Bridge struct {
	BridgeCaller     // Read-only binding to the contract
	BridgeTransactor // Write-only binding to the contract
	BridgeFilterer   // Log filterer for contract events
}

// BridgeCaller is an auto generated read-only Go binding around an Ethereum contract.
type BridgeCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BridgeTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BridgeTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BridgeFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BridgeFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BridgeSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BridgeSession struct {
	Contract     *Bridge           // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BridgeCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BridgeCallerSession struct {
	Contract *BridgeCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts // Call options to use throughout this session
}

// BridgeTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BridgeTransactorSession struct {
	Contract     *BridgeTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BridgeRaw is an auto generated low-level Go binding around an Ethereum contract.
type BridgeRaw struct {
	Contract *Bridge // Generic contract binding to access the raw methods on
}

// BridgeCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BridgeCallerRaw struct {
	Contract *BridgeCaller // Generic read-only contract binding to access the raw methods on
}

// BridgeTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BridgeTransactorRaw struct {
	Contract *BridgeTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBridge creates a new instance of Bridge, bound to a specific deployed contract.
func NewBridge(address common.Address, backend bind.ContractBackend) (*Bridge, error) {
	contract, err := bindBridge(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Bridge{BridgeCaller: BridgeCaller{contract: contract}, BridgeTransactor: BridgeTransactor{contract: contract}, BridgeFilterer: BridgeFilterer{contract: contract}}, nil
}

// NewBridgeCaller creates a new read-only instance of Bridge, bound to a specific deployed contract.
func NewBridgeCaller(address common.Address, caller bind.ContractCaller) (*BridgeCaller, error) {
	contract, err := bindBridge(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BridgeCaller{contract: contract}, nil
}

// NewBridgeTransactor creates a new write-only instance of Bridge, bound to a specific deployed contract.
func NewBridgeTransactor(address common.Address, transactor bind.ContractTransactor) (*BridgeTransactor, error) {
	contract, err := bindBridge(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BridgeTransactor{contract: contract}, nil
}

// NewBridgeFilterer creates a new log filterer instance of Bridge, bound to a specific deployed contract.
func NewBridgeFilterer(address common.Address, filterer bind.ContractFilterer) (*BridgeFilterer, error) {
	contract, err := bindBridge(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BridgeFilterer{contract: contract}, nil
}

// bindBridge binds a generic wrapper to an already deployed contract.
func bindBridge(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := BridgeMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Bridge *BridgeRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Bridge.Contract.BridgeCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Bridge *BridgeRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Bridge.Contract.BridgeTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Bridge *BridgeRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Bridge.Contract.BridgeTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Bridge *BridgeCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Bridge.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Bridge *BridgeTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Bridge.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Bridge *BridgeTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Bridge.Contract.contract.Transact(opts, method, params...)
}

// ComputeReclaimEffects is a free data retrieval call binding the contract method 0x566d54c6.
//
// Solidity: function compute_reclaim_effects((bytes32,uint256,uint8,bytes)[] sourceAllocations, (bytes32,uint256,uint8,bytes)[] targetAllocations, uint256 indexOfTargetInSource) pure returns((bytes32,uint256,uint8,bytes)[])
func (_Bridge *BridgeCaller) ComputeReclaimEffects(opts *bind.CallOpts, sourceAllocations []ExitFormatAllocation, targetAllocations []ExitFormatAllocation, indexOfTargetInSource *big.Int) ([]ExitFormatAllocation, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "compute_reclaim_effects", sourceAllocations, targetAllocations, indexOfTargetInSource)

	if err != nil {
		return *new([]ExitFormatAllocation), err
	}

	out0 := *abi.ConvertType(out[0], new([]ExitFormatAllocation)).(*[]ExitFormatAllocation)

	return out0, err

}

// ComputeReclaimEffects is a free data retrieval call binding the contract method 0x566d54c6.
//
// Solidity: function compute_reclaim_effects((bytes32,uint256,uint8,bytes)[] sourceAllocations, (bytes32,uint256,uint8,bytes)[] targetAllocations, uint256 indexOfTargetInSource) pure returns((bytes32,uint256,uint8,bytes)[])
func (_Bridge *BridgeSession) ComputeReclaimEffects(sourceAllocations []ExitFormatAllocation, targetAllocations []ExitFormatAllocation, indexOfTargetInSource *big.Int) ([]ExitFormatAllocation, error) {
	return _Bridge.Contract.ComputeReclaimEffects(&_Bridge.CallOpts, sourceAllocations, targetAllocations, indexOfTargetInSource)
}

// ComputeReclaimEffects is a free data retrieval call binding the contract method 0x566d54c6.
//
// Solidity: function compute_reclaim_effects((bytes32,uint256,uint8,bytes)[] sourceAllocations, (bytes32,uint256,uint8,bytes)[] targetAllocations, uint256 indexOfTargetInSource) pure returns((bytes32,uint256,uint8,bytes)[])
func (_Bridge *BridgeCallerSession) ComputeReclaimEffects(sourceAllocations []ExitFormatAllocation, targetAllocations []ExitFormatAllocation, indexOfTargetInSource *big.Int) ([]ExitFormatAllocation, error) {
	return _Bridge.Contract.ComputeReclaimEffects(&_Bridge.CallOpts, sourceAllocations, targetAllocations, indexOfTargetInSource)
}

// ComputeTransferEffectsAndInteractions is a free data retrieval call binding the contract method 0x11e9f178.
//
// Solidity: function compute_transfer_effects_and_interactions(uint256 initialHoldings, (bytes32,uint256,uint8,bytes)[] allocations, uint256[] indices) pure returns((bytes32,uint256,uint8,bytes)[] newAllocations, bool allocatesOnlyZeros, (bytes32,uint256,uint8,bytes)[] exitAllocations, uint256 totalPayouts)
func (_Bridge *BridgeCaller) ComputeTransferEffectsAndInteractions(opts *bind.CallOpts, initialHoldings *big.Int, allocations []ExitFormatAllocation, indices []*big.Int) (struct {
	NewAllocations     []ExitFormatAllocation
	AllocatesOnlyZeros bool
	ExitAllocations    []ExitFormatAllocation
	TotalPayouts       *big.Int
}, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "compute_transfer_effects_and_interactions", initialHoldings, allocations, indices)

	outstruct := new(struct {
		NewAllocations     []ExitFormatAllocation
		AllocatesOnlyZeros bool
		ExitAllocations    []ExitFormatAllocation
		TotalPayouts       *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.NewAllocations = *abi.ConvertType(out[0], new([]ExitFormatAllocation)).(*[]ExitFormatAllocation)
	outstruct.AllocatesOnlyZeros = *abi.ConvertType(out[1], new(bool)).(*bool)
	outstruct.ExitAllocations = *abi.ConvertType(out[2], new([]ExitFormatAllocation)).(*[]ExitFormatAllocation)
	outstruct.TotalPayouts = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// ComputeTransferEffectsAndInteractions is a free data retrieval call binding the contract method 0x11e9f178.
//
// Solidity: function compute_transfer_effects_and_interactions(uint256 initialHoldings, (bytes32,uint256,uint8,bytes)[] allocations, uint256[] indices) pure returns((bytes32,uint256,uint8,bytes)[] newAllocations, bool allocatesOnlyZeros, (bytes32,uint256,uint8,bytes)[] exitAllocations, uint256 totalPayouts)
func (_Bridge *BridgeSession) ComputeTransferEffectsAndInteractions(initialHoldings *big.Int, allocations []ExitFormatAllocation, indices []*big.Int) (struct {
	NewAllocations     []ExitFormatAllocation
	AllocatesOnlyZeros bool
	ExitAllocations    []ExitFormatAllocation
	TotalPayouts       *big.Int
}, error) {
	return _Bridge.Contract.ComputeTransferEffectsAndInteractions(&_Bridge.CallOpts, initialHoldings, allocations, indices)
}

// ComputeTransferEffectsAndInteractions is a free data retrieval call binding the contract method 0x11e9f178.
//
// Solidity: function compute_transfer_effects_and_interactions(uint256 initialHoldings, (bytes32,uint256,uint8,bytes)[] allocations, uint256[] indices) pure returns((bytes32,uint256,uint8,bytes)[] newAllocations, bool allocatesOnlyZeros, (bytes32,uint256,uint8,bytes)[] exitAllocations, uint256 totalPayouts)
func (_Bridge *BridgeCallerSession) ComputeTransferEffectsAndInteractions(initialHoldings *big.Int, allocations []ExitFormatAllocation, indices []*big.Int) (struct {
	NewAllocations     []ExitFormatAllocation
	AllocatesOnlyZeros bool
	ExitAllocations    []ExitFormatAllocation
	TotalPayouts       *big.Int
}, error) {
	return _Bridge.Contract.ComputeTransferEffectsAndInteractions(&_Bridge.CallOpts, initialHoldings, allocations, indices)
}

// Holdings is a free data retrieval call binding the contract method 0x166e56cd.
//
// Solidity: function holdings(address , bytes32 ) view returns(uint256)
func (_Bridge *BridgeCaller) Holdings(opts *bind.CallOpts, arg0 common.Address, arg1 [32]byte) (*big.Int, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "holdings", arg0, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// Holdings is a free data retrieval call binding the contract method 0x166e56cd.
//
// Solidity: function holdings(address , bytes32 ) view returns(uint256)
func (_Bridge *BridgeSession) Holdings(arg0 common.Address, arg1 [32]byte) (*big.Int, error) {
	return _Bridge.Contract.Holdings(&_Bridge.CallOpts, arg0, arg1)
}

// Holdings is a free data retrieval call binding the contract method 0x166e56cd.
//
// Solidity: function holdings(address , bytes32 ) view returns(uint256)
func (_Bridge *BridgeCallerSession) Holdings(arg0 common.Address, arg1 [32]byte) (*big.Int, error) {
	return _Bridge.Contract.Holdings(&_Bridge.CallOpts, arg0, arg1)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Bridge *BridgeCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Bridge *BridgeSession) Owner() (common.Address, error) {
	return _Bridge.Contract.Owner(&_Bridge.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Bridge *BridgeCallerSession) Owner() (common.Address, error) {
	return _Bridge.Contract.Owner(&_Bridge.CallOpts)
}

// StatusOf is a free data retrieval call binding the contract method 0xc7df14e2.
//
// Solidity: function statusOf(bytes32 ) view returns(bytes32)
func (_Bridge *BridgeCaller) StatusOf(opts *bind.CallOpts, arg0 [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _Bridge.contract.Call(opts, &out, "statusOf", arg0)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// StatusOf is a free data retrieval call binding the contract method 0xc7df14e2.
//
// Solidity: function statusOf(bytes32 ) view returns(bytes32)
func (_Bridge *BridgeSession) StatusOf(arg0 [32]byte) ([32]byte, error) {
	return _Bridge.Contract.StatusOf(&_Bridge.CallOpts, arg0)
}

// StatusOf is a free data retrieval call binding the contract method 0xc7df14e2.
//
// Solidity: function statusOf(bytes32 ) view returns(bytes32)
func (_Bridge *BridgeCallerSession) StatusOf(arg0 [32]byte) ([32]byte, error) {
	return _Bridge.Contract.StatusOf(&_Bridge.CallOpts, arg0)
}

// Deposit is a paid mutator transaction binding the contract method 0x2fb1d270.
//
// Solidity: function deposit(address asset, bytes32 channelId, uint256 expectedHeld, uint256 amount) payable returns()
func (_Bridge *BridgeTransactor) Deposit(opts *bind.TransactOpts, asset common.Address, channelId [32]byte, expectedHeld *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "deposit", asset, channelId, expectedHeld, amount)
}

// Deposit is a paid mutator transaction binding the contract method 0x2fb1d270.
//
// Solidity: function deposit(address asset, bytes32 channelId, uint256 expectedHeld, uint256 amount) payable returns()
func (_Bridge *BridgeSession) Deposit(asset common.Address, channelId [32]byte, expectedHeld *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _Bridge.Contract.Deposit(&_Bridge.TransactOpts, asset, channelId, expectedHeld, amount)
}

// Deposit is a paid mutator transaction binding the contract method 0x2fb1d270.
//
// Solidity: function deposit(address asset, bytes32 channelId, uint256 expectedHeld, uint256 amount) payable returns()
func (_Bridge *BridgeTransactorSession) Deposit(asset common.Address, channelId [32]byte, expectedHeld *big.Int, amount *big.Int) (*types.Transaction, error) {
	return _Bridge.Contract.Deposit(&_Bridge.TransactOpts, asset, channelId, expectedHeld, amount)
}

// Reclaim is a paid mutator transaction binding the contract method 0xb89659e3.
//
// Solidity: function reclaim((bytes32,(address[],uint64,address,uint48),((address,(uint8,bytes),(bytes32,uint256,uint8,bytes)[])[],bytes,uint48,bool),bytes,uint256,uint256,bytes32,bytes,uint256) reclaimArgs) returns()
func (_Bridge *BridgeTransactor) Reclaim(opts *bind.TransactOpts, reclaimArgs IMultiAssetHolderReclaimArgs) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "reclaim", reclaimArgs)
}

// Reclaim is a paid mutator transaction binding the contract method 0xb89659e3.
//
// Solidity: function reclaim((bytes32,(address[],uint64,address,uint48),((address,(uint8,bytes),(bytes32,uint256,uint8,bytes)[])[],bytes,uint48,bool),bytes,uint256,uint256,bytes32,bytes,uint256) reclaimArgs) returns()
func (_Bridge *BridgeSession) Reclaim(reclaimArgs IMultiAssetHolderReclaimArgs) (*types.Transaction, error) {
	return _Bridge.Contract.Reclaim(&_Bridge.TransactOpts, reclaimArgs)
}

// Reclaim is a paid mutator transaction binding the contract method 0xb89659e3.
//
// Solidity: function reclaim((bytes32,(address[],uint64,address,uint48),((address,(uint8,bytes),(bytes32,uint256,uint8,bytes)[])[],bytes,uint48,bool),bytes,uint256,uint256,bytes32,bytes,uint256) reclaimArgs) returns()
func (_Bridge *BridgeTransactorSession) Reclaim(reclaimArgs IMultiAssetHolderReclaimArgs) (*types.Transaction, error) {
	return _Bridge.Contract.Reclaim(&_Bridge.TransactOpts, reclaimArgs)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Bridge *BridgeTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Bridge *BridgeSession) RenounceOwnership() (*types.Transaction, error) {
	return _Bridge.Contract.RenounceOwnership(&_Bridge.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Bridge *BridgeTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _Bridge.Contract.RenounceOwnership(&_Bridge.TransactOpts)
}

// Transfer is a paid mutator transaction binding the contract method 0x3033730e.
//
// Solidity: function transfer(uint256 assetIndex, bytes32 fromChannelId, bytes outcomeBytes, bytes32 stateHash, uint256[] indices) returns()
func (_Bridge *BridgeTransactor) Transfer(opts *bind.TransactOpts, assetIndex *big.Int, fromChannelId [32]byte, outcomeBytes []byte, stateHash [32]byte, indices []*big.Int) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "transfer", assetIndex, fromChannelId, outcomeBytes, stateHash, indices)
}

// Transfer is a paid mutator transaction binding the contract method 0x3033730e.
//
// Solidity: function transfer(uint256 assetIndex, bytes32 fromChannelId, bytes outcomeBytes, bytes32 stateHash, uint256[] indices) returns()
func (_Bridge *BridgeSession) Transfer(assetIndex *big.Int, fromChannelId [32]byte, outcomeBytes []byte, stateHash [32]byte, indices []*big.Int) (*types.Transaction, error) {
	return _Bridge.Contract.Transfer(&_Bridge.TransactOpts, assetIndex, fromChannelId, outcomeBytes, stateHash, indices)
}

// Transfer is a paid mutator transaction binding the contract method 0x3033730e.
//
// Solidity: function transfer(uint256 assetIndex, bytes32 fromChannelId, bytes outcomeBytes, bytes32 stateHash, uint256[] indices) returns()
func (_Bridge *BridgeTransactorSession) Transfer(assetIndex *big.Int, fromChannelId [32]byte, outcomeBytes []byte, stateHash [32]byte, indices []*big.Int) (*types.Transaction, error) {
	return _Bridge.Contract.Transfer(&_Bridge.TransactOpts, assetIndex, fromChannelId, outcomeBytes, stateHash, indices)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Bridge *BridgeTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Bridge *BridgeSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.TransferOwnership(&_Bridge.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Bridge *BridgeTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.TransferOwnership(&_Bridge.TransactOpts, newOwner)
}

// UpdateMirroredChannelStates is a paid mutator transaction binding the contract method 0x7837f977.
//
// Solidity: function updateMirroredChannelStates(bytes32 channelId, bytes32 stateHash, bytes outcomeBytes, uint256 amount, address asset) returns()
func (_Bridge *BridgeTransactor) UpdateMirroredChannelStates(opts *bind.TransactOpts, channelId [32]byte, stateHash [32]byte, outcomeBytes []byte, amount *big.Int, asset common.Address) (*types.Transaction, error) {
	return _Bridge.contract.Transact(opts, "updateMirroredChannelStates", channelId, stateHash, outcomeBytes, amount, asset)
}

// UpdateMirroredChannelStates is a paid mutator transaction binding the contract method 0x7837f977.
//
// Solidity: function updateMirroredChannelStates(bytes32 channelId, bytes32 stateHash, bytes outcomeBytes, uint256 amount, address asset) returns()
func (_Bridge *BridgeSession) UpdateMirroredChannelStates(channelId [32]byte, stateHash [32]byte, outcomeBytes []byte, amount *big.Int, asset common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateMirroredChannelStates(&_Bridge.TransactOpts, channelId, stateHash, outcomeBytes, amount, asset)
}

// UpdateMirroredChannelStates is a paid mutator transaction binding the contract method 0x7837f977.
//
// Solidity: function updateMirroredChannelStates(bytes32 channelId, bytes32 stateHash, bytes outcomeBytes, uint256 amount, address asset) returns()
func (_Bridge *BridgeTransactorSession) UpdateMirroredChannelStates(channelId [32]byte, stateHash [32]byte, outcomeBytes []byte, amount *big.Int, asset common.Address) (*types.Transaction, error) {
	return _Bridge.Contract.UpdateMirroredChannelStates(&_Bridge.TransactOpts, channelId, stateHash, outcomeBytes, amount, asset)
}

// BridgeAllocationUpdatedIterator is returned from FilterAllocationUpdated and is used to iterate over the raw logs and unpacked data for AllocationUpdated events raised by the Bridge contract.
type BridgeAllocationUpdatedIterator struct {
	Event *BridgeAllocationUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *BridgeAllocationUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeAllocationUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(BridgeAllocationUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *BridgeAllocationUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeAllocationUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeAllocationUpdated represents a AllocationUpdated event raised by the Bridge contract.
type BridgeAllocationUpdated struct {
	ChannelId       [32]byte
	Asset           common.Address
	AssetIndex      *big.Int
	InitialHoldings *big.Int
	FinalHoldings   *big.Int
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterAllocationUpdated is a free log retrieval operation binding the contract event 0x95655fb00939f9d12257c78a601be335cd6ce1ce12296e2f367918fcf25fe4e3.
//
// Solidity: event AllocationUpdated(bytes32 indexed channelId, address asset, uint256 assetIndex, uint256 initialHoldings, uint256 finalHoldings)
func (_Bridge *BridgeFilterer) FilterAllocationUpdated(opts *bind.FilterOpts, channelId [][32]byte) (*BridgeAllocationUpdatedIterator, error) {

	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "AllocationUpdated", channelIdRule)
	if err != nil {
		return nil, err
	}
	return &BridgeAllocationUpdatedIterator{contract: _Bridge.contract, event: "AllocationUpdated", logs: logs, sub: sub}, nil
}

// WatchAllocationUpdated is a free log subscription operation binding the contract event 0x95655fb00939f9d12257c78a601be335cd6ce1ce12296e2f367918fcf25fe4e3.
//
// Solidity: event AllocationUpdated(bytes32 indexed channelId, address asset, uint256 assetIndex, uint256 initialHoldings, uint256 finalHoldings)
func (_Bridge *BridgeFilterer) WatchAllocationUpdated(opts *bind.WatchOpts, sink chan<- *BridgeAllocationUpdated, channelId [][32]byte) (event.Subscription, error) {

	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "AllocationUpdated", channelIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeAllocationUpdated)
				if err := _Bridge.contract.UnpackLog(event, "AllocationUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAllocationUpdated is a log parse operation binding the contract event 0x95655fb00939f9d12257c78a601be335cd6ce1ce12296e2f367918fcf25fe4e3.
//
// Solidity: event AllocationUpdated(bytes32 indexed channelId, address asset, uint256 assetIndex, uint256 initialHoldings, uint256 finalHoldings)
func (_Bridge *BridgeFilterer) ParseAllocationUpdated(log types.Log) (*BridgeAllocationUpdated, error) {
	event := new(BridgeAllocationUpdated)
	if err := _Bridge.contract.UnpackLog(event, "AllocationUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeDepositedIterator is returned from FilterDeposited and is used to iterate over the raw logs and unpacked data for Deposited events raised by the Bridge contract.
type BridgeDepositedIterator struct {
	Event *BridgeDeposited // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *BridgeDepositedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeDeposited)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(BridgeDeposited)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *BridgeDepositedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeDepositedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeDeposited represents a Deposited event raised by the Bridge contract.
type BridgeDeposited struct {
	Destination         [32]byte
	Asset               common.Address
	DestinationHoldings *big.Int
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterDeposited is a free log retrieval operation binding the contract event 0x87d4c0b5e30d6808bc8a94ba1c4d839b29d664151551a31753387ee9ef48429b.
//
// Solidity: event Deposited(bytes32 indexed destination, address asset, uint256 destinationHoldings)
func (_Bridge *BridgeFilterer) FilterDeposited(opts *bind.FilterOpts, destination [][32]byte) (*BridgeDepositedIterator, error) {

	var destinationRule []interface{}
	for _, destinationItem := range destination {
		destinationRule = append(destinationRule, destinationItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "Deposited", destinationRule)
	if err != nil {
		return nil, err
	}
	return &BridgeDepositedIterator{contract: _Bridge.contract, event: "Deposited", logs: logs, sub: sub}, nil
}

// WatchDeposited is a free log subscription operation binding the contract event 0x87d4c0b5e30d6808bc8a94ba1c4d839b29d664151551a31753387ee9ef48429b.
//
// Solidity: event Deposited(bytes32 indexed destination, address asset, uint256 destinationHoldings)
func (_Bridge *BridgeFilterer) WatchDeposited(opts *bind.WatchOpts, sink chan<- *BridgeDeposited, destination [][32]byte) (event.Subscription, error) {

	var destinationRule []interface{}
	for _, destinationItem := range destination {
		destinationRule = append(destinationRule, destinationItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "Deposited", destinationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeDeposited)
				if err := _Bridge.contract.UnpackLog(event, "Deposited", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDeposited is a log parse operation binding the contract event 0x87d4c0b5e30d6808bc8a94ba1c4d839b29d664151551a31753387ee9ef48429b.
//
// Solidity: event Deposited(bytes32 indexed destination, address asset, uint256 destinationHoldings)
func (_Bridge *BridgeFilterer) ParseDeposited(log types.Log) (*BridgeDeposited, error) {
	event := new(BridgeDeposited)
	if err := _Bridge.contract.UnpackLog(event, "Deposited", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Bridge contract.
type BridgeOwnershipTransferredIterator struct {
	Event *BridgeOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *BridgeOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(BridgeOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *BridgeOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeOwnershipTransferred represents a OwnershipTransferred event raised by the Bridge contract.
type BridgeOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Bridge *BridgeFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*BridgeOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &BridgeOwnershipTransferredIterator{contract: _Bridge.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Bridge *BridgeFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *BridgeOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeOwnershipTransferred)
				if err := _Bridge.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Bridge *BridgeFilterer) ParseOwnershipTransferred(log types.Log) (*BridgeOwnershipTransferred, error) {
	event := new(BridgeOwnershipTransferred)
	if err := _Bridge.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeReclaimedIterator is returned from FilterReclaimed and is used to iterate over the raw logs and unpacked data for Reclaimed events raised by the Bridge contract.
type BridgeReclaimedIterator struct {
	Event *BridgeReclaimed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *BridgeReclaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeReclaimed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(BridgeReclaimed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *BridgeReclaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeReclaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeReclaimed represents a Reclaimed event raised by the Bridge contract.
type BridgeReclaimed struct {
	ChannelId  [32]byte
	AssetIndex *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterReclaimed is a free log retrieval operation binding the contract event 0x4d3754632451ebba9812a9305e7bca17b67a17186a5cff93d2e9ae1b01e3d27b.
//
// Solidity: event Reclaimed(bytes32 indexed channelId, uint256 assetIndex)
func (_Bridge *BridgeFilterer) FilterReclaimed(opts *bind.FilterOpts, channelId [][32]byte) (*BridgeReclaimedIterator, error) {

	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "Reclaimed", channelIdRule)
	if err != nil {
		return nil, err
	}
	return &BridgeReclaimedIterator{contract: _Bridge.contract, event: "Reclaimed", logs: logs, sub: sub}, nil
}

// WatchReclaimed is a free log subscription operation binding the contract event 0x4d3754632451ebba9812a9305e7bca17b67a17186a5cff93d2e9ae1b01e3d27b.
//
// Solidity: event Reclaimed(bytes32 indexed channelId, uint256 assetIndex)
func (_Bridge *BridgeFilterer) WatchReclaimed(opts *bind.WatchOpts, sink chan<- *BridgeReclaimed, channelId [][32]byte) (event.Subscription, error) {

	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "Reclaimed", channelIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeReclaimed)
				if err := _Bridge.contract.UnpackLog(event, "Reclaimed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseReclaimed is a log parse operation binding the contract event 0x4d3754632451ebba9812a9305e7bca17b67a17186a5cff93d2e9ae1b01e3d27b.
//
// Solidity: event Reclaimed(bytes32 indexed channelId, uint256 assetIndex)
func (_Bridge *BridgeFilterer) ParseReclaimed(log types.Log) (*BridgeReclaimed, error) {
	event := new(BridgeReclaimed)
	if err := _Bridge.contract.UnpackLog(event, "Reclaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BridgeStatusUpdatedIterator is returned from FilterStatusUpdated and is used to iterate over the raw logs and unpacked data for StatusUpdated events raised by the Bridge contract.
type BridgeStatusUpdatedIterator struct {
	Event *BridgeStatusUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *BridgeStatusUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BridgeStatusUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(BridgeStatusUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *BridgeStatusUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BridgeStatusUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BridgeStatusUpdated represents a StatusUpdated event raised by the Bridge contract.
type BridgeStatusUpdated struct {
	ChannelId [32]byte
	StateHash [32]byte
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterStatusUpdated is a free log retrieval operation binding the contract event 0x62e6bf5c61a11078212ba836ebb8494a794e81614008017cced73376a0892aa5.
//
// Solidity: event StatusUpdated(bytes32 indexed channelId, bytes32 stateHash)
func (_Bridge *BridgeFilterer) FilterStatusUpdated(opts *bind.FilterOpts, channelId [][32]byte) (*BridgeStatusUpdatedIterator, error) {

	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}

	logs, sub, err := _Bridge.contract.FilterLogs(opts, "StatusUpdated", channelIdRule)
	if err != nil {
		return nil, err
	}
	return &BridgeStatusUpdatedIterator{contract: _Bridge.contract, event: "StatusUpdated", logs: logs, sub: sub}, nil
}

// WatchStatusUpdated is a free log subscription operation binding the contract event 0x62e6bf5c61a11078212ba836ebb8494a794e81614008017cced73376a0892aa5.
//
// Solidity: event StatusUpdated(bytes32 indexed channelId, bytes32 stateHash)
func (_Bridge *BridgeFilterer) WatchStatusUpdated(opts *bind.WatchOpts, sink chan<- *BridgeStatusUpdated, channelId [][32]byte) (event.Subscription, error) {

	var channelIdRule []interface{}
	for _, channelIdItem := range channelId {
		channelIdRule = append(channelIdRule, channelIdItem)
	}

	logs, sub, err := _Bridge.contract.WatchLogs(opts, "StatusUpdated", channelIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BridgeStatusUpdated)
				if err := _Bridge.contract.UnpackLog(event, "StatusUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseStatusUpdated is a log parse operation binding the contract event 0x62e6bf5c61a11078212ba836ebb8494a794e81614008017cced73376a0892aa5.
//
// Solidity: event StatusUpdated(bytes32 indexed channelId, bytes32 stateHash)
func (_Bridge *BridgeFilterer) ParseStatusUpdated(log types.Log) (*BridgeStatusUpdated, error) {
	event := new(BridgeStatusUpdated)
	if err := _Bridge.contract.UnpackLog(event, "StatusUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
