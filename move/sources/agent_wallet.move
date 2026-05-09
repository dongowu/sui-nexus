/// Agent Wallet with self-enforcing policy: budget caps, protocol scope,
/// time windows, on-chain activity logs, and owner revocation.
/// Built for Sui Overflow 2026 Agentic Web track.
module sui_nexus::agent_wallet {
    use sui::object::{Self, UID};
    use sui::tx_context::{Self, TxContext};
    use sui::transfer;
    use sui::event;
    use sui::coin::{Self, Coin};
    use sui::balance::{Self, Balance};
    use std::string::String;
    use std::vector;

    // ═══════════════════════════════════════════════
    // Errors
    // ═══════════════════════════════════════════════
    const ENotAuthorized: u64 = 1;
    const EWalletRevoked: u64 = 2;
    const ENotYetActive: u64 = 3;
    const EExpired: u64 = 4;
    const EProtocolNotAllowed: u64 = 5;
    const EBudgetExceeded: u64 = 6;
    const EInsufficientBalance: u64 = 7;
    const ENotOwner: u64 = 8;

    // ═══════════════════════════════════════════════
    // Structs
    // ═══════════════════════════════════════════════

    /// A single activity log entry, stored on-chain for auditability.
    public struct ActivityEntry has copy, drop, store {
        timestamp: u64,
        action: String,
        amount_mist: u64,
        protocol: String,
        description: String,
    }

    /// The Agent Wallet — a shared object enforcing policy on every operation.
    /// Stores SUI balance internally so policy checks and fund transfers are atomic.
    public struct AgentWallet has key, store {
        id: UID,
        owner: address,
        agent_address: address,
        budget_cap_mist: u64,
        budget_spent_mist: u64,
        time_start: u64, // epoch when wallet becomes active
        time_end: u64,   // epoch when wallet expires
        allowed_protocols: vector<String>, // empty = all protocols allowed
        is_active: bool,
        activity_log: vector<ActivityEntry>,
        balance: Balance<SUI>,
    }

    // ═══════════════════════════════════════════════
    // Events
    // ═══════════════════════════════════════════════

    public struct WalletCreated has copy, drop {
        wallet_id: address,
        owner: address,
        agent_address: address,
        budget_cap_mist: u64,
        time_start: u64,
        time_end: u64,
    }

    public struct TradeExecuted has copy, drop {
        wallet_id: address,
        agent: address,
        action: String,
        amount_mist: u64,
        protocol: String,
        budget_remaining: u64,
    }

    public struct WalletRevoked has copy, drop {
        wallet_id: address,
        owner: address,
        total_spent: u64,
    }

    public struct WalletDeposited has copy, drop {
        wallet_id: address,
        amount: u64,
        new_balance: u64,
    }

    // ═══════════════════════════════════════════════
    // Owner Functions
    // ═══════════════════════════════════════════════

    /// Create a new AgentWallet with the given policy parameters.
    /// - `agent_address`: the zkLogin-derived address of the authorized agent
    /// - `budget_cap_mist`: maximum total spend in MIST
    /// - `allowed_protocols`: protocol package IDs the agent can interact with
    /// - `time_end`: epoch when the wallet expires
    /// The wallet starts active from the current epoch.
    public fun create_wallet(
        agent_address: address,
        budget_cap_mist: u64,
        allowed_protocols: vector<String>,
        time_end: u64,
        ctx: &mut TxContext,
    ) {
        let owner = tx_context::sender(ctx);
        let time_start = tx_context::epoch(ctx);
        let wallet = AgentWallet {
            id: object::new(ctx),
            owner,
            agent_address,
            budget_cap_mist,
            budget_spent_mist: 0,
            time_start,
            time_end,
            allowed_protocols,
            is_active: true,
            activity_log: vector::empty(),
            balance: balance::zero(),
        };

        event::emit(WalletCreated {
            wallet_id: object::uid_to_address(&wallet.id),
            owner,
            agent_address,
            budget_cap_mist,
            time_start,
            time_end,
        });

        // Make shared so both owner and agent can access
        transfer::share_object(wallet);
    }

    /// Owner deposits SUI into the wallet.
    public fun deposit(wallet: &mut AgentWallet, coin: Coin<SUI>, _ctx: &TxContext) {
        assert!(wallet.is_active, EWalletRevoked);
        let amount = coin::value(&coin);
        balance::join(&mut wallet.balance, coin::into_balance(coin));

        event::emit(WalletDeposited {
            wallet_id: object::uid_to_address(&wallet.id),
            amount,
            new_balance: balance::value(&wallet.balance),
        });
    }

    /// Owner revokes the wallet, permanently freezing all agent activity.
    public fun revoke(wallet: &mut AgentWallet, ctx: &TxContext) {
        assert!(tx_context::sender(ctx) == wallet.owner, ENotOwner);
        wallet.is_active = false;

        event::emit(WalletRevoked {
            wallet_id: object::uid_to_address(&wallet.id),
            owner: wallet.owner,
            total_spent: wallet.budget_spent_mist,
        });
    }

    /// Owner withdraws all remaining SUI from a revoked wallet.
    public fun withdraw(wallet: &mut AgentWallet, ctx: &mut TxContext): Coin<SUI> {
        assert!(tx_context::sender(ctx) == wallet.owner, ENotOwner);
        assert!(!wallet.is_active, 0); // wallet must be revoked first

        let amount = balance::value(&wallet.balance);
        let coin = coin::take(&mut wallet.balance, amount, ctx);
        coin
    }

    // ═══════════════════════════════════════════════
    // Agent Functions
    // ═══════════════════════════════════════════════

    /// Execute a trade through the Agent Wallet.
    /// Enforces ALL policy constraints atomically:
    /// 1. Wallet is active (not revoked)
    /// 2. Current epoch is within the time window
    /// 3. Protocol is in the allowlist (if non-empty)
    /// 4. Amount does not exceed remaining budget
    /// 5. Wallet has sufficient balance
    ///
    /// On success: deducts budget, logs activity, emits TradeExecuted event,
    /// and returns the approved Coin<SUI> for the agent to use.
    public fun execute_trade(
        wallet: &mut AgentWallet,
        amount_mist: u64,
        protocol: String,
        description: String,
        ctx: &mut TxContext,
    ): Coin<SUI> {
        // 1. Active check
        assert!(wallet.is_active, EWalletRevoked);

        // 2. Time window check
        let epoch = tx_context::epoch(ctx);
        assert!(epoch >= wallet.time_start, ENotYetActive);
        assert!(epoch <= wallet.time_end, EExpired);

        // 3. Protocol allowlist check
        if (!vector::is_empty(&wallet.allowed_protocols)) {
            let found = false;
            let i = 0;
            let len = vector::length(&wallet.allowed_protocols);
            while (i < len) {
                let allowed = vector::borrow(&wallet.allowed_protocols, i);
                if (allowed == &protocol) {
                    found = true;
                    break
                };
                i = i + 1;
            };
            assert!(found, EProtocolNotAllowed);
        };

        // 4. Budget cap check
        let new_spent = wallet.budget_spent_mist + amount_mist;
        assert!(new_spent <= wallet.budget_cap_mist, EBudgetExceeded);

        // 5. Balance check
        assert!(balance::value(&wallet.balance) >= amount_mist, EInsufficientBalance);

        // Deduct budget
        wallet.budget_spent_mist = new_spent;

        // Split the approved amount from wallet balance
        let approved_coin = coin::take(&mut wallet.balance, amount_mist, ctx);

        // Log activity
        vector::push_back(&mut wallet.activity_log, ActivityEntry {
            timestamp: epoch,
            action: string::utf8(b"trade"),
            amount_mist,
            protocol,
            description,
        });

        // Emit event
        event::emit(TradeExecuted {
            wallet_id: object::uid_to_address(&wallet.id),
            agent: tx_context::sender(ctx),
            action: string::utf8(b"trade"),
            amount_mist,
            protocol,
            budget_remaining: wallet.budget_cap_mist - wallet.budget_spent_mist,
        });

        approved_coin
    }

    // ═══════════════════════════════════════════════
    // Query Functions
    // ═══════════════════════════════════════════════

    /// Return the remaining budget in MIST.
    public fun budget_remaining(wallet: &AgentWallet): u64 {
        wallet.budget_cap_mist - wallet.budget_spent_mist
    }

    /// Check if the wallet is still active.
    public fun is_active(wallet: &AgentWallet): bool {
        wallet.is_active
    }

    /// Return the wallet's current balance.
    public fun wallet_balance(wallet: &AgentWallet): u64 {
        balance::value(&wallet.balance)
    }

    /// Return the wallet owner address.
    public fun owner(wallet: &AgentWallet): address {
        wallet.owner
    }

    /// Return the authorized agent address.
    public fun agent(wallet: &AgentWallet): address {
        wallet.agent_address
    }

    /// Return the number of activity log entries.
    public fun activity_count(wallet: &AgentWallet): u64 {
        vector::length(&wallet.activity_log) as u64
    }
}
