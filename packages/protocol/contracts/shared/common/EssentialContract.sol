// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import "@openzeppelin/contracts-upgradeable/access/Ownable2StepUpgradeable.sol";
import "./AddressResolver.sol";
import "./IResolver.sol";

/// @title EssentialContract
/// @custom:security-contact security@taiko.xyz
abstract contract EssentialContract is UUPSUpgradeable, Ownable2StepUpgradeable {
    uint8 internal constant _FALSE = 1;
    uint8 internal constant _TRUE = 2;

    IResolver public resolver;
    uint256[49] private __gap0;

    /// @dev Slot 1.
    uint8 internal __reentry;
    uint8 internal __paused;
    uint64 internal __lastUnpausedAt;

    uint256[49] private __gap1;

    /// @notice Emitted when the contract is paused.
    /// @param account The account that paused the contract.
    event Paused(address account);

    /// @notice Emitted when the contract is unpaused.
    /// @param account The account that unpaused the contract.
    event Unpaused(address account);

    error INVALID_PAUSE_STATUS();
    error FUNC_NOT_IMPLEMENTED();
    error REENTRANT_CALL();
    error RESOLVER_DENIED();
    error ZERO_ADDRESS();
    error ZERO_VALUE();

    /// @dev Modifier that ensures the caller is the owner or resolved address of a given name.
    /// @param _name The name to check against.
    modifier onlyFromOwnerOrNamed(bytes32 _name) {
        require(msg.sender == owner() || msg.sender == resolve(_name, true), RESOLVER_DENIED());
        _;
    }

    modifier notImplemented() {
        revert FUNC_NOT_IMPLEMENTED();
        _;
    }

    modifier nonReentrant() {
        require(_loadReentryLock() != _TRUE, REENTRANT_CALL());
        _storeReentryLock(_TRUE);
        _;
        _storeReentryLock(_FALSE);
    }

    modifier whenPaused() {
        require(paused(), INVALID_PAUSE_STATUS());
        _;
    }

    modifier whenNotPaused() {
        require(!paused(), INVALID_PAUSE_STATUS());
        _;
    }

    modifier nonZeroAddr(address _addr) {
        require(_addr != address(0), ZERO_ADDRESS());
        _;
    }

    modifier nonZeroValue(uint256 _value) {
        require(_value != 0, ZERO_VALUE());
        _;
    }

    modifier nonZeroBytes32(bytes32 _value) {
        require(_value != 0, ZERO_VALUE());
        _;
    }

    /// @dev Modifier that ensures the caller is the resolved address of a given
    /// name.
    /// @param _name The name to check against.
    modifier onlyFromNamed(bytes32 _name) {
        require(msg.sender == resolve(_name, true), RESOLVER_DENIED());
        _;
    }

    /// @dev Modifier that ensures the caller is the resolved address of a given
    /// name, if the name is set.
    /// @param _name The name to check against.
    modifier onlyFromOptionalNamed(bytes32 _name) {
        address addr = resolve(_name, true);
        require(addr == address(0) || msg.sender == addr, RESOLVER_DENIED());
        _;
    }

    /// @dev Modifier that ensures the caller is a resolved address to either _name1 or _name2
    /// name.
    /// @param _name1 The first name to check against.
    /// @param _name2 The second name to check against.
    modifier onlyFromNamedEither(bytes32 _name1, bytes32 _name2) {
        require(
            msg.sender == resolve(_name1, true) || msg.sender == resolve(_name2, true),
            RESOLVER_DENIED()
        );
        _;
    }

    // TODO
    constructor( /*IResolver _resolver*/ ) {
        // resolver = IResolver(address(0));
        _disableInitializers();
    }

    /// @notice Pauses the contract.
    function pause() public virtual {
        _pause();
        // We call the authorize function here to avoid:
        // Warning (5740): Unreachable code.
        _authorizePause(msg.sender, true);
    }

    /// @notice Unpauses the contract.
    function unpause() public virtual {
        _unpause();
        // We call the authorize function here to avoid:
        // Warning (5740): Unreachable code.
        _authorizePause(msg.sender, false);
    }

    function impl() public view returns (address) {
        return _getImplementation();
    }

    /// @notice Returns true if the contract is paused, and false otherwise.
    /// @return true if paused, false otherwise.
    function paused() public view returns (bool) {
        return __paused == _TRUE;
    }

    function lastUnpausedAt() public view virtual returns (uint64) {
        return __lastUnpausedAt;
    }

    function inNonReentrant() public view returns (bool) {
        return _loadReentryLock() == _TRUE;
    }

    function resolve(
        uint64 _chainId,
        bytes32 _name,
        bool _allowZeroAddress
    )
        public
        view
        returns (address)
    {
        return resolver.resolve(_chainId, _name, _allowZeroAddress);
    }

    function resolve(bytes32 _name, bool _allowZeroAddress) public view returns (address) {
        return resolver.resolve(uint64(block.chainid), _name, _allowZeroAddress);
    }

    /// @notice Initializes the contract.
    /// @param _owner The owner of this contract. msg.sender will be used if this value is zero.
    /// @param _resolver The address of the {AddressManager} contract.
    function __Essential_init(address _owner, address _resolver) internal nonZeroAddr(_resolver) {
        __Essential_init(_owner);
        resolver = IResolver(_resolver);
    }

    function __Essential_init(address _owner) internal virtual onlyInitializing {
        __Context_init();
        _transferOwnership(_owner == address(0) ? msg.sender : _owner);
        __paused = _FALSE;
    }

    function _pause() internal whenNotPaused {
        __paused = _TRUE;
        emit Paused(msg.sender);
    }

    function _unpause() internal whenPaused {
        __paused = _FALSE;
        __lastUnpausedAt = uint64(block.timestamp);
        emit Unpaused(msg.sender);
    }

    function _authorizeUpgrade(address) internal virtual override onlyOwner { }

    function _authorizePause(address, bool) internal virtual onlyOwner { }

    // Stores the reentry lock
    function _storeReentryLock(uint8 _reentry) internal virtual {
        __reentry = _reentry;
    }

    // Loads the reentry lock
    function _loadReentryLock() internal view virtual returns (uint8 reentry_) {
        reentry_ = __reentry;
    }
}
