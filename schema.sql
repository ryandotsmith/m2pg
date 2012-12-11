create table metrics(
  id uuid,
  bucket int,
  name text,
  count numeric,
  mean numeric,
  median numeric,
  min numeric,
  max numeric,
  perc95 numeric,
  perc99 numeric,
  last numeric
);

create index metrics_by_id on metrics(id);
create index metrics_by_name_bucket on metrics(name, bucket);
