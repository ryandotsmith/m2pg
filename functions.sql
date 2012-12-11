drop function if exists metrics(text, int, int, text);
create or replace function metrics(text, timestamptz, timestamptz, text)
returns TABLE(
  id uuid,
  name text,
  bucket text,
  count numeric,
  mean numeric,
  median numeric,
  min numeric,
  max numeric,
  perc95 numeric,
  perc99 numeric,
  last numeric
)
as $$
  select
    id uuid,
    name,
    date_trunc($4, bucket)::text as bucket,
    sum(count) as count,
    avg(mean) as mean,
    max(median) as median,
    min(min) as min,
    max(max) as max,
    max(perc95) as perc95,
    max(perc99) as perc99,
    max(last) as last
  from metrics
  where name ~ $1 and bucket >= $2 and bucket <= $3
  group by id, name, date_trunc($4, bucket)
  order by date_trunc($4, bucket) desc
$$ language sql immutable;
