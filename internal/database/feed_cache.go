// Copyright (c) 2021 - 2023, Ludvig Lundgren and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/autobrr/autobrr/internal/domain"
	"github.com/autobrr/autobrr/internal/logger"
	"github.com/autobrr/autobrr/pkg/errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/rs/zerolog"
)

type FeedCacheRepo struct {
	log zerolog.Logger
	db  *DB
}

func NewFeedCacheRepo(log logger.Logger, db *DB) domain.FeedCacheRepo {
	return &FeedCacheRepo{
		log: log.With().Str("module", "database").Str("repo", "feed_cache").Logger(),
		db:  db,
	}
}

func (r *FeedCacheRepo) Get(bucket string, key string) ([]byte, error) {
	queryBuilder := r.db.squirrel.
		Select(
			"value",
			"ttl",
		).
		From("feed_cache").
		Where(sq.Eq{"bucket": bucket}).
		Where(sq.Eq{"key": key}).
		Where(sq.Gt{"ttl": time.Now()})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	row := r.db.handler.QueryRow(query, args...)
	if err := row.Err(); err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}

	var value []byte
	var ttl time.Duration

	if err := row.Scan(&value, &ttl); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "error scanning row")
	}

	return value, nil
}

func (r *FeedCacheRepo) GetByBucket(ctx context.Context, bucket string) ([]domain.FeedCacheItem, error) {
	queryBuilder := r.db.squirrel.
		Select(
			"bucket",
			"key",
			"value",
			"ttl",
		).
		From("feed_cache").
		Where(sq.Eq{"bucket": bucket})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "error building query")
	}

	rows, err := r.db.handler.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}

	defer rows.Close()

	var data []domain.FeedCacheItem

	for rows.Next() {
		var d domain.FeedCacheItem

		if err := rows.Scan(&d.Bucket, &d.Key, &d.Value, &d.TTL); err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}

		data = append(data, d)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row error")
	}

	return data, nil
}

func (r *FeedCacheRepo) GetCountByBucket(ctx context.Context, bucket string) (int, error) {

	queryBuilder := r.db.squirrel.
		Select("COUNT(*)").
		From("feed_cache").
		Where(sq.Eq{"bucket": bucket})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "error building query")
	}

	row := r.db.handler.QueryRowContext(ctx, query, args...)
	if err != nil {
		return 0, errors.Wrap(err, "error executing query")
	}

	var count = 0

	if err := row.Scan(&count); err != nil {
		return 0, errors.Wrap(err, "error scanning row")
	}

	return count, nil
}

func (r *FeedCacheRepo) Exists(bucket string, key string) (bool, error) {
	queryBuilder := r.db.squirrel.
		Select("1").
		Prefix("SELECT EXISTS (").
		From("feed_cache").
		Where(sq.Eq{"bucket": bucket}).
		Where(sq.Eq{"key": key}).
		Suffix(")")

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return false, errors.Wrap(err, "error building query")
	}

	var exists bool
	err = r.db.handler.QueryRow(query, args...).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, domain.ErrRecordNotFound
		}

		return false, errors.Wrap(err, "error query")
	}

	return exists, nil
}

func (r *FeedCacheRepo) Put(bucket string, key string, val []byte, ttl time.Time) error {
	queryBuilder := r.db.squirrel.
		Insert("feed_cache").
		Columns("bucket", "key", "value", "ttl").
		Values(bucket, key, val, ttl)

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building query")
	}

	if _, err = r.db.handler.Exec(query, args...); err != nil {
		return errors.Wrap(err, "error executing query")
	}

	return nil
}

func (r *FeedCacheRepo) Delete(ctx context.Context, bucket string, key string) error {
	queryBuilder := r.db.squirrel.
		Delete("feed_cache").
		Where(sq.Eq{"bucket": bucket}).
		Where(sq.Eq{"key": key})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building query")
	}

	_, err = r.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "error executing query")
	}

	return nil
}

func (r *FeedCacheRepo) DeleteBucket(ctx context.Context, bucket string) error {
	queryBuilder := r.db.squirrel.
		Delete("feed_cache").
		Where(sq.Eq{"bucket": bucket})

	query, args, err := queryBuilder.ToSql()
	if err != nil {
		return errors.Wrap(err, "error building query")
	}

	result, err := r.db.handler.ExecContext(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "error executing query")
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "error exec result")
	}

	if rows == 0 {
		return errors.Wrap(err, "error no rows affected")
	}

	return nil
}
