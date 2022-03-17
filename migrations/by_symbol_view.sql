create or replace view by_symbol as
select sym,
       sum(profit_part) profit,
       count(profit_part) count,
       sum(bet_volume) volume,
       (sum(profit_part)*100/sum(bet_volume))::numeric(9,4) avg_profit

from surebet_view
group by sym
order by count(*) desc
;

select *
from surebet_view;