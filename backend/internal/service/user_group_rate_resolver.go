package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

type userGroupRateConfigReader interface {
	GetRateConfigByUserAndGroup(ctx context.Context, userID, groupID int64) (*UserGroupRateConfig, error)
}

type userGroupRateResolver struct {
	repo         UserGroupRateRepository
	cache        *gocache.Cache
	cacheTTL     time.Duration
	sf           *singleflight.Group
	logComponent string
}

func newUserGroupRateResolver(repo UserGroupRateRepository, cache *gocache.Cache, cacheTTL time.Duration, sf *singleflight.Group, logComponent string) *userGroupRateResolver {
	if cacheTTL <= 0 {
		cacheTTL = defaultUserGroupRateCacheTTL
	}
	if cache == nil {
		cache = gocache.New(cacheTTL, time.Minute)
	}
	if logComponent == "" {
		logComponent = "service.gateway"
	}
	if sf == nil {
		sf = &singleflight.Group{}
	}

	return &userGroupRateResolver{
		repo:         repo,
		cache:        cache,
		cacheTTL:     cacheTTL,
		sf:           sf,
		logComponent: logComponent,
	}
}

func (r *userGroupRateResolver) Resolve(ctx context.Context, userID, groupID int64, groupDefaultMultiplier float64) float64 {
	cfg := r.resolveConfig(ctx, userID, groupID)
	if cfg == nil {
		return groupDefaultMultiplier
	}
	return cfg.RateMultiplier
}

func (r *userGroupRateResolver) ResolveActual(ctx context.Context, userID, groupID int64, groupActualMultiplier float64) float64 {
	cfg := r.resolveConfig(ctx, userID, groupID)
	if cfg == nil {
		return groupActualMultiplier
	}
	if cfg.ActualRateMultiplier != nil {
		return *cfg.ActualRateMultiplier
	}
	if cfg.RateMultiplier >= 0 {
		return cfg.RateMultiplier
	}
	return groupActualMultiplier
}

func (r *userGroupRateResolver) resolveConfig(ctx context.Context, userID, groupID int64) *UserGroupRateConfig {
	if r == nil || userID <= 0 || groupID <= 0 {
		return nil
	}

	key := fmt.Sprintf("%d:%d", userID, groupID)
	if r.cache != nil {
		if cached, ok := r.cache.Get(key); ok {
			if cfg, castOK := cached.(*UserGroupRateConfig); castOK {
				userGroupRateCacheHitTotal.Add(1)
				return cfg
			}
		}
	}
	if r.repo == nil {
		return nil
	}
	userGroupRateCacheMissTotal.Add(1)

	value, err, shared := r.sf.Do(key, func() (any, error) {
		if r.cache != nil {
			if cached, ok := r.cache.Get(key); ok {
				if cfg, castOK := cached.(*UserGroupRateConfig); castOK {
					userGroupRateCacheHitTotal.Add(1)
					return cfg, nil
				}
			}
		}

		userGroupRateCacheLoadTotal.Add(1)
		cfgReader, ok := r.repo.(userGroupRateConfigReader)
		if !ok {
			userRate, repoErr := r.repo.GetByUserAndGroup(ctx, userID, groupID)
			if repoErr != nil {
				return nil, repoErr
			}
			if userRate == nil {
				if r.cache != nil {
					r.cache.Set(key, (*UserGroupRateConfig)(nil), r.cacheTTL)
				}
				return (*UserGroupRateConfig)(nil), nil
			}
			cfg := &UserGroupRateConfig{RateMultiplier: *userRate}
			if r.cache != nil {
				r.cache.Set(key, cfg, r.cacheTTL)
			}
			return cfg, nil
		}

		cfg, repoErr := cfgReader.GetRateConfigByUserAndGroup(ctx, userID, groupID)
		if repoErr != nil {
			return nil, repoErr
		}
		if r.cache != nil {
			r.cache.Set(key, cfg, r.cacheTTL)
		}
		return cfg, nil
	})
	if shared {
		userGroupRateCacheSFSharedTotal.Add(1)
	}
	if err != nil {
		userGroupRateCacheFallbackTotal.Add(1)
		logger.LegacyPrintf(r.logComponent, "get user group rate failed, fallback to group default: user=%d group=%d err=%v", userID, groupID, err)
		return nil
	}

	cfg, ok := value.(*UserGroupRateConfig)
	if !ok {
		userGroupRateCacheFallbackTotal.Add(1)
		return nil
	}
	return cfg
}
