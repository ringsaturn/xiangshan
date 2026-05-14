INSTALL spatial; LOAD spatial;

COPY (
  SELECT
    da.id                        AS area_id,
    da.division_id               AS division_id,
    da.subtype                   AS subtype,
    COALESCE(da.admin_level, -1) AS admin_level,
    da.country                   AS country,
    da.region                    AS region,
    da.names['primary']          AS name,
    CAST(to_json(da.names['common']) AS VARCHAR) AS names_common,
    d.parent_division_id         AS parent_id,
    da.class                     AS class,
    d.wikidata                   AS wikidata,
    d.population                 AS population,
    d.norms.driving_side         AS driving_side,
    d.local_type['en']           AS local_type,
    da.bbox.xmin                 AS xmin,
    da.bbox.xmax                 AS xmax,
    da.bbox.ymin                 AS ymin,
    da.bbox.ymax                 AS ymax,
    ST_AsWKB(da.geometry)        AS geometry_wkb
  FROM
    read_parquet('data/divisions/type=division_area/*.zstd.parquet') AS da
    LEFT JOIN read_parquet('data/divisions/type=division/*.zstd.parquet') AS d
      ON da.division_id = d.id
  WHERE
    da.subtype IN (
      'country', 'dependency',
      'macroregion', 'region',
      'macrocounty', 'county',
      'localadmin', 'locality'
    )
    AND da.is_territorial = true
    AND da.names['primary'] IS NOT NULL
  ORDER BY da.subtype, da.country, da.admin_level
) TO 'build/extracted.parquet' (FORMAT parquet, COMPRESSION zstd);
