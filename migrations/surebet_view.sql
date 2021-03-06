drop view surebet_view;

-- create or replace view surebet_view as
select s.created_at surebet_created,
--        surebets.id,
--        last_bin_time,
--        start_time,
--        begin_place,
--        surebets.done,
--        ftx_spread,
--        bin_spread,
--        buy_profit,
--        sell_profit,
       target_profit,
--        profit,
       amount_coef,
       target_amount,
       base_usd_value::int,

--        s.profit_inc,

--        ((s.base_usd_value/s.max_stake-s.target_amount)*s.profit_inc)::numeric(9,4)              amount_coef_2,
--        profit_inc,
       s.volume bet_volume,
--        max_stake,
--        s.place_price,
--        surebets.place_type,
       s.place_size                     bet_place,
--        surebets.place_ioc,
--        surebets.place_post_only,
--        bin_symbol,
--        bin_bid_price,
--        bin_bid_qty,
--        bin_ask_price,
--        bin_ask_qty,
--        bin_server_time,
--        bin_receive_time,
       ftx_symbol                       sym,
       s.place_side                     si,

--        ftx_bid_price,
--        ftx_bid_qty,
--        ftx_ask_price,
--        ftx_ask_qty,
--        ftx_server_time,
--        ftx_receive_time,
--        base_free,
--        base_total,
--        quote_free,
--        quote_total,
--        quote_usd_value,
--        maker_fee,
--        taker_fee,
--        base_currency,
--        quote_currency,
--        min_provide_size,
--        size_increment,
--        price_increment,
--        change1_h,
--        change24_h,
--        change_bod,
--        quote_volume24_h,
--        volume_usd24_h,
--        profit_price_diff,
--        conn_reused,
--        bin_volume,
--        price,
--        profit_sub_spread,
--        bin_price,
       profit_sub_fee,
       required_profit,

--        real_fee,
--        bin_prev_bid_price,
--        bin_prev_bid_qty,
--        bin_prev_ask_price,
--        bin_prev_ask_qty,
--        bin_prev_server_time,
--        bin_prev_receive_time,
--        ftx_prev_bid_price,
--        ftx_prev_bid_qty,
--        ftx_prev_ask_price,
--        ftx_prev_ask_qty,
--        ftx_prev_server_time,
--        ftx_prev_receive_time,
--        surebets.order_id,
--        avg_price_diff,
--        max_price_diff,
--        min_price_diff,
--        profit_sub_avg,
--        heals.created_at,
--        heals.id,
--        start,
--        heals.done,
--        heals.order_id,
--        heals.place_side,
--        heals.place_type,
--        heals.place_size,
--        heals.place_ioc,
--        heals.place_post_only,
       h.filled_size                    bet_fill_size,
       h.avg_fill_price::numeric(10, 5) bet_fill_price,
--        ho.price,

--        h.place_price::numeric(10, 5),
       fee_part::numeric(9, 3),
       profit_part::numeric(9, 4),
--        ho.created_at,
       h.place_side                          heal_side,
       ho.status                        heal_status,
--        type,
       ho.client_id                     heal_client_id,
--        ho.price,
       ho.avg_fill_price                heal_fill_price,
       ho.size                          heal_size,
--        ho.closed_at,
       ho.filled_size                   heal_fill_size,
       h.error_msg,
       (h.done - h.start)/1000000 heal_elapsed_ms
from surebets s
         left join orders bo on bo.id = s.order_id
         left join heals h on s.id = h.id
         left join orders ho on ho.id = h.order_id
where bo.filled_size > 0
order by s.created_at desc;

