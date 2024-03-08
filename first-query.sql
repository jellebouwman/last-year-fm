-- artist
-- release_group
-- release_group_meta
select
  ac.name as artist_name,
  rg.name as recording_group_name,
  rgm.first_release_date_year as release_year
from
  artist_credit as ac
  left join release_group as rg on ac.id = rg.artist_credit
  left join release_group_meta as rgm on rgm.id = rg.id;