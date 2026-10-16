package commerce

import (
	"context"
	"time"

	"rangoapp/internal/easystock/catalog"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Service) CreateCatalogProduct(ctx context.Context, p tenant.Principal, in catalog.CreateInput) (catalog.Product, error) {
	var out catalog.Product
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.Create(ctx, tx, p, in)
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.product", "product", out.ID, map[string]string{"sku": out.SKU})
	})
	return out, err
}

func (s *Service) UpdateCatalogProduct(ctx context.Context, p tenant.Principal, id uuid.UUID, in catalog.UpdateInput) (catalog.Product, error) {
	var out catalog.Product
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.Update(ctx, tx, p, id, in)
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.product", "product", out.ID, map[string]string{"sku": out.SKU})
	})
	return out, err
}

func (s *Service) SetProductDensity(ctx context.Context, p tenant.Principal, in catalog.DensityInput) (catalog.Product, error) {
	var out catalog.Product
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.SetDensity(ctx, tx, p, in)
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.density", "product", out.ID, map[string]bool{"enabled": in.Enabled})
	})
	return out, err
}

func (s *Service) ArchiveCatalogProduct(ctx context.Context, p tenant.Principal, id uuid.UUID) (catalog.Product, error) {
	var out catalog.Product
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.Archive(ctx, tx, p, id)
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.archive", "product", out.ID, map[string]string{"sku": out.SKU})
	})
	return out, err
}

func (s *Service) CreateCatalogUnit(ctx context.Context, p tenant.Principal, productID uuid.UUID, in catalog.UnitInput) (catalog.Unit, error) {
	var out catalog.Unit
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.CreateUnit(ctx, tx, p, productID, in)
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.unit", "product_unit", out.ID, map[string]string{"name": out.Name, "factor": out.Factor})
	})
	return out, err
}

func (s *Service) UpdateCatalogUnit(ctx context.Context, p tenant.Principal, unitID uuid.UUID, in catalog.UnitInput) (catalog.Unit, error) {
	var out catalog.Unit
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.UpdateUnit(ctx, tx, p, unitID, in)
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.unit", "product_unit", out.ID, map[string]string{"name": out.Name, "factor": out.Factor})
	})
	return out, err
}

func (s *Service) ArchiveCatalogUnit(ctx context.Context, p tenant.Principal, unitID uuid.UUID) (catalog.Unit, error) {
	var out catalog.Unit
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.ArchiveUnit(ctx, tx, p, unitID)
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.unit", "product_unit", out.ID, map[string]string{"active": "false"})
	})
	return out, err
}

func (s *Service) UpsertCatalogPrice(ctx context.Context, p tenant.Principal, in catalog.PriceInput) (catalog.Price, error) {
	var out catalog.Price
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.UpsertPrice(ctx, tx, p, in)
		if err != nil {
			return err
		}
		return audit(ctx, tx, p, &out.StoreID, "catalog.price", "product", out.ProductID, map[string]string{
			"currency": out.Currency, "unitPrice": out.UnitPrice,
		})
	})
	return out, err
}

func (s *Service) ArchiveCatalogPrice(ctx context.Context, p tenant.Principal, id uuid.UUID) error {
	return s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		if err := catalog.ArchivePrice(ctx, tx, p, id); err != nil {
			return err
		}
		return audit(ctx, tx, p, nil, "catalog.price", "product_price", id, map[string]string{"archived": "true"})
	})
}

func (s *Service) ListProducts(ctx context.Context, p tenant.Principal, filter catalog.ListFilter) ([]catalog.Product, error) {
	var out []catalog.Product
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.List(ctx, tx, p, filter)
		return err
	})
	return out, err
}

func (s *Service) GetProduct(ctx context.Context, p tenant.Principal, id, storeID uuid.UUID) (catalog.Product, error) {
	var out catalog.Product
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.Get(ctx, tx, p, id, storeID)
		return err
	})
	return out, err
}

func (s *Service) ProductByBarcode(ctx context.Context, p tenant.Principal, storeID uuid.UUID, barcode string) (catalog.Product, error) {
	var out catalog.Product
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.ByBarcode(ctx, tx, p, storeID, barcode)
		return err
	})
	return out, err
}

func (s *Service) ProductCategories(ctx context.Context, p tenant.Principal) ([]catalog.Category, error) {
	var out []catalog.Category
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.Categories(ctx, tx, p)
		return err
	})
	return out, err
}

func (s *Service) CatalogChanges(ctx context.Context, p tenant.Principal, after time.Time, afterID, storeID uuid.UUID, limit int) ([]catalog.Product, error) {
	var out []catalog.Product
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		out, err = catalog.ChangesForStore(ctx, tx, p, after, afterID, storeID, limit)
		return err
	})
	return out, err
}

func (s *Service) ProductCost(ctx context.Context, p tenant.Principal, storeID, productID uuid.UUID) (*catalog.Cost, error) {
	var out *catalog.Cost
	err := s.withTx(ctx, p, func(ctx context.Context, tx pgx.Tx) error {
		visible, err := catalog.CostVisible(ctx, tx, p, storeID)
		if err != nil || !visible {
			return err
		}
		out, err = catalog.LoadCost(ctx, tx, storeID, productID)
		return err
	})
	return out, err
}
