// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import "../../contracts/common/LibStrings.sol";
import "../../contracts/L1/tiers/ITierProvider.sol";

/// @title TestTierProvider
/// @dev Labeled in AddressResolver as "tier_provider"
/// @custom:security-contact security@taiko.xyz
contract TestTierProvider is ITierProvider {
    uint256[50] private __gap;

    /// @inheritdoc ITierProvider
    function getTier(
        uint256, /*_blockId*/
        uint16 _tierId
    )
        public
        pure
        override
        returns (ITierProvider.Tier memory)
    {
        if (_tierId == LibTiers.TIER_OPTIMISTIC) {
            return ITierProvider.Tier({
                verifierName: "",
                validityBond: 250 ether, // TKO
                contestBond: 500 ether, // TKO
                cooldownWindow: 1440, //24 hours
                provingWindow: 30, // 0.5 hours
                maxBlocksToVerifyPerProof: 12
            });
        }

        if (_tierId == LibTiers.TIER_SGX) {
            return ITierProvider.Tier({
                verifierName: LibStrings.B_TIER_SGX,
                validityBond: 250 ether, // TKO
                contestBond: 1640 ether, // =250TKO * 6.5625
                cooldownWindow: 1440, //24 hours
                provingWindow: 60, // 1 hours
                maxBlocksToVerifyPerProof: 8
            });
        }

        if (_tierId == LibTiers.TIER_GUARDIAN) {
            return ITierProvider.Tier({
                verifierName: LibStrings.B_TIER_GUARDIAN,
                validityBond: 0, // must be 0 for top tier
                contestBond: 0, // must be 0 for top tier
                cooldownWindow: 60, //1 hours
                provingWindow: 2880, // 48 hours
                maxBlocksToVerifyPerProof: 16
            });
        }

        revert TIER_NOT_FOUND();
    }

    /// @inheritdoc ITierProvider
    function getTierIds(uint256 /*_blockId*/ )
        public
        pure
        override
        returns (uint16[] memory tiers_)
    {
        tiers_ = new uint16[](3);
        tiers_[0] = LibTiers.TIER_OPTIMISTIC;
        tiers_[1] = LibTiers.TIER_SGX;
        tiers_[2] = LibTiers.TIER_GUARDIAN;
    }

    /// @inheritdoc ITierProvider
    function getMinTier(
        uint256, /*_blockId*/
        uint256 _rand
    )
        public
        pure
        override
        returns (uint16)
    {
        // 10% will be selected to require SGX proofs.
        if (_rand % 10 == 0) return LibTiers.TIER_SGX;
        // Other blocks are optimistic, without validity proofs.
        return LibTiers.TIER_OPTIMISTIC;
    }
}
