# Data Consumer Demo


Note the "is ConsumerBase", to extend
https://github.com/unification-com/xfund-router/blob/main/contracts/lib/ConsumerBase.sol
ConsumerBase.sol interacts with the deployed Router.sol smart contract
which will route data requests and fee payment to the selected provider
and handle data fulfilment.

The selected provider will listen to the Router for requests, then send the data
back to the Router, which will in turn forward the data to your smart contract
after verifying the source of the data.

## Functions:
- [`constructor(address _router, address _xfund, address _provider, uint256 _fee) public`](#DemoConsumer2-constructor-address-address-address-uint256-)
- [`setProvider(address _provider) external`](#DemoConsumer2-setProvider-address-)
- [`setFee(uint256 _fee) external`](#DemoConsumer2-setFee-uint256-)
- [`requestData(bytes32 _data, string pair) external`](#DemoConsumer2-requestData-bytes32-string-)
- [`getPrice(string pair) external`](#DemoConsumer2-getPrice-string-)
- [`increaseRouterAllowance(uint256 _amount) external`](#DemoConsumer2-increaseRouterAllowance-uint256-)
- [`setRouter(address _router) external`](#DemoConsumer2-setRouter-address-)
- [`withdrawxFund(address _to, uint256 _value) external`](#DemoConsumer2-withdrawxFund-address-uint256-)
- [`receiveData(uint256 _price, bytes32 _requestId) internal`](#DemoConsumer2-receiveData-uint256-bytes32-)



<a name="DemoConsumer2-constructor-address-address-address-uint256-"></a>
### Function `constructor(address _router, address _xfund, address _provider, uint256 _fee) public `
constructor must pass the address of the Router and xFUND smart
contracts to the constructor of your contract! Without it, this contract
cannot interact with the system, nor request/receive any data.
The constructor calls the parent ConsumerBase constructor to set these.


#### Parameters:
- `_router`: address of the Router smart contract

- `_xfund`: address of the xFUND smart contract

- `_provider`: address of the default provider

- `_fee`: uint256 default fee
<a name="DemoConsumer2-setProvider-address-"></a>
### Function `setProvider(address _provider) external `
setProvider change default provider. Uses OpenZeppelin's
onlyOwner modifier to secure the function.


#### Parameters:
- `_provider`: address of the default provider
<a name="DemoConsumer2-setFee-uint256-"></a>
### Function `setFee(uint256 _fee) external `
setFee change default fee. Uses OpenZeppelin's
onlyOwner modifier to secure the function.


#### Parameters:
- `_fee`: uint256 default fee
<a name="DemoConsumer2-requestData-bytes32-string-"></a>
### Function `requestData(bytes32 _data, string pair) external `
getData the actual function to request data.

NOTE: Calls ConsumerBase.sol's requestData function.

Uses OpenZeppelin's onlyOwner modifier to secure the function.
The data format can be found at
https://docs.finchains.io/guide/ooo_api.html
Endpoints should be Hex encoded using, for example
the web3.utils.asciiToHex function.


#### Parameters:
- `_data`: bytes32 data being requested.
<a name="DemoConsumer2-getPrice-string-"></a>
### Function `getPrice(string pair) external  -> uint256`
getPrice returns the current price

#### Return Values:
- price uint256
<a name="DemoConsumer2-increaseRouterAllowance-uint256-"></a>
### Function `increaseRouterAllowance(uint256 _amount) external `
increaseRouterAllowance allows the Router to spend xFUND on behalf of this
smart contract.

NOTE: Calls the internal _increaseRouterAllowance function in ConsumerBase.sol.

Required so that xFUND fees can be paid. Uses OpenZeppelin's onlyOwner modifier
to secure the function.


#### Parameters:
- `_amount`: uint256 amount to increase
<a name="DemoConsumer2-setRouter-address-"></a>
### Function `setRouter(address _router) external `
setRouter allows updating the Router contract address

NOTE: Calls the internal setRouter function in ConsumerBase.sol.

Can be used if network upgrades require new Router deployments.
Uses OpenZeppelin's onlyOwner modifier to secure the function.


#### Parameters:
- `_router`: address new Router address
<a name="DemoConsumer2-withdrawxFund-address-uint256-"></a>
### Function `withdrawxFund(address _to, uint256 _value) external `
increaseRouterAllowance allows contract owner to withdraw
any xFUND held in this contract.
Uses OpenZeppelin's onlyOwner modifier to secure the function.


#### Parameters:
- `_to`: address recipient

- `_value`: uint256 amount to withdraw
<a name="DemoConsumer2-receiveData-uint256-bytes32-"></a>
### Function `receiveData(uint256 _price, bytes32 _requestId) internal `
recieveData - example end user function to receive data. This will be called
by the data provider, via the Router's fulfillRequest, and through the ConsumerBase's
rawReceiveData function.

Note: validation of the data and data provider sending the data is handled
by the Router smart contract prior to it forwarding the data to your contract, allowing
devs to focus on pure functionality. ConsumerBase.sol's rawReceiveData
function can only be called by the Router smart contract.


#### Parameters:
- `_price`: uint256 result being sent

- `_requestId`: bytes32 request ID of the request being fulfilled


