-- create or replace view balance_view as
select coin,
       free,
       total,
       usd_value::int,
       available_without_borrow                                    available,
       (usd_value / total)::numeric(9, 3)                          price,

       ((usd_value / 28) - 18)::numeric(9, 3)                      norm_count,

       (0.03 + ((usd_value / 28) - 17) * 0.03 / 17)::numeric(9, 4) new_buy,
       (0.03 - ((usd_value / 28) - 17) * 0.03 / 17)::numeric(9, 4) new_sell,
       sum(usd_value) over () /
       (select count(coin) from balances where usd_value > 0 and coin != 'USDT' and coin != 'USD'),
       (select count(coin) from balances where usd_value > 0 and coin != 'USDT' and coin != 'USD'),
       count(coin) over ()
from balances
where usd_value > 0
  and coin != 'USDT'
order by usd_value desc;


select coin_count, coin_sum,
       coin,
       free,
       total,
       usd_value::int,
       (usd_value / total)::numeric(9, 3)                          price,
    ((usd_value / stake) - coin_count)::numeric(9, 3)                      norm_count

--        available_without_borrow,
from (
         select count(coin) coin_count,
                sum(usd_value) coin_sum,
                28 stake
         from balances
         where usd_value > 0
           and coin != 'USDT'
           and coin != 'USD'
     ) t,
     balances
where usd_value > 0
