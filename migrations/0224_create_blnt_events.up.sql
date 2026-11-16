-- 0224 up — typed BLNT V3 event projection.
--
-- One row per event accepted by the contract-identity-gated BLNT V3 decoder.
-- BLNT V3 is Blend-derived but has its own pool, tiered-backstop, emitter and
-- backfill ABIs; these rows intentionally do not enter the blend_* tables.
-- Common semantic fields are promoted to typed columns, while decoded
-- event-specific structures (reserve configuration, auction legs and emitter
-- drop recipients) live in attributes. Raw XDR is not duplicated here: it is
-- already retained in the lake and the shared soroban_events landing zone and
-- can be replayed into this projection.
--
-- Amounts are raw, unscaled i128 values stored as NUMERIC. There is no
-- retention policy; compression after 30 days limits storage growth.

BEGIN;

CREATE TABLE blnt_events (
    contract_id       text        NOT NULL,
    ledger            integer     NOT NULL CHECK (ledger >= 0),
    ledger_close_time timestamptz NOT NULL,
    tx_hash           char(64)    NOT NULL,
    op_index          integer     NOT NULL CHECK (op_index >= 0),
    event_index       integer     NOT NULL CHECK (event_index >= 0),

    contract_role text NOT NULL CHECK (contract_role IN (
        'pool', 'factory', 'backstop', 'emitter', 'backfill', 'lockup_backfill'
    )),
    event_kind text NOT NULL,

    pool         text,
    user_address text,
    asset        text,
    counterparty text,
    target       text,
    admin        text,
    tier         text CHECK (tier IN ('first_loss', 'second_loss', 'third_loss')),
    auction_type smallint CHECK (auction_type BETWEEN 0 AND 3),

    amount  numeric,
    amount2 numeric,
    amount3 numeric,
    amount4 numeric,

    attributes jsonb NOT NULL DEFAULT '{}'::jsonb
        CHECK (jsonb_typeof(attributes) = 'object'),

    derive_generation bigint      NOT NULL DEFAULT 0,
    ingested_at       timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT blnt_events_role_kind CHECK (
        CASE contract_role
            WHEN 'pool' THEN event_kind IN (
                'set_admin', 'update_pool', 'queue_set_reserve',
                'cancel_set_reserve', 'set_reserve', 'set_status',
                'reserve_emission_update', 'gulp_emissions', 'claim',
                'bad_debt', 'defaulted_debt', 'supply', 'withdraw',
                'supply_collateral', 'withdraw_collateral', 'borrow', 'repay',
                'flash_loan', 'clawback', 'gulp', 'reconcile_loss',
                'protocol_credit_loss', 'new_auction', 'fill_auction',
                'delete_auction'
            )
            WHEN 'factory' THEN event_kind = 'deploy'
            WHEN 'backstop' THEN event_kind IN (
                'migration_activated', 'backfill_funded', 'deposit',
                'queue_withdrawal', 'dequeue_withdrawal', 'withdraw',
                'distribute', 'gulp_emissions', 'rw_zone_add',
                'rw_zone_remove', 'claim', 'draw', 'donate'
            )
            WHEN 'emitter' THEN event_kind IN (
                'distribute', 'q_swap', 'del_swap', 'swap', 'drop'
            )
            WHEN 'backfill' THEN event_kind IN (
                'claim', 'claim_grant', 'swap_blnd', 'burn_expired'
            )
            WHEN 'lockup_backfill' THEN event_kind IN (
                'fund', 'claim', 'claim_fund', 'expire'
            )
            ELSE FALSE
        END
    ),

    PRIMARY KEY (ledger_close_time, contract_id, ledger, tx_hash,
                 op_index, event_index)
);

COMMENT ON TABLE blnt_events IS
    'Typed, identity-gated BLNT V3 events. Common semantic fields are columns; '
    'attributes contains decoded event-specific structures. Raw XDR remains in '
    'the lake and soroban_events. Not a Blend V1/V2 table or spot-price source.';

SELECT create_hypertable(
    'blnt_events',
    'ledger_close_time',
    chunk_time_interval => INTERVAL '30 days',
    if_not_exists       => TRUE
);

CREATE INDEX blnt_events_contract_ts_idx
    ON blnt_events (contract_id, ledger_close_time DESC);

CREATE INDEX blnt_events_role_kind_ts_idx
    ON blnt_events (contract_role, event_kind, ledger_close_time DESC);

CREATE INDEX blnt_events_tx_hash_idx
    ON blnt_events (tx_hash, ledger_close_time DESC);

ALTER TABLE blnt_events SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'contract_role, event_kind',
    timescaledb.compress_orderby   = 'ledger_close_time DESC, ledger DESC'
);

SELECT add_compression_policy(
    'blnt_events',
    INTERVAL '30 days',
    if_not_exists => TRUE
);

COMMIT;
