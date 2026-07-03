package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/pisah/share"
)

var errNotFound = errors.New("not found")

type Store struct{ pool *pgxpool.Pool }

// ---- domain types ----

type Split struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Merchant    string     `json:"merchant"`
	OwnerID     string     `json:"-"`
	OwnerName   string     `json:"ownerName"`
	OwnerQRURL  *string    `json:"ownerQrUrl"`
	CapturedAt  *time.Time `json:"capturedAt"`
	SubtotalSen int64      `json:"subtotalSen"`
	SSTSen      int64      `json:"sstSen"`
	ServiceSen  int64      `json:"serviceSen"`
	RoundingSen int64      `json:"roundingSen"`
	TotalSen    int64      `json:"totalSen"`
	CreatedAt   *time.Time `json:"createdAt"`
}

type SplitSummary struct {
	Split        Split  `json:"split"`
	ShareURL     string `json:"shareUrl"`
	CollectedSen int64  `json:"collectedSen"`
}

func (s Split) TaxTotalSen() int64 { return s.SSTSen + s.ServiceSen + s.RoundingSen }

type Item struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Qty             int      `json:"qty"`
	UnitPriceSen    int64    `json:"unitPriceSen"`
	LineTotalSen    int64    `json:"lineTotalSen"`
	Position        int      `json:"position"`
	IncludedInSplit bool     `json:"includedInSplit"`
	Claimants       int      `json:"claimants"` // how many participants claimed it
	ClaimedBy       []string `json:"claimedBy"` // claimant names, for the friend UI
}

type Participant struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	IsOwner bool       `json:"isOwner"`
	OwedSen int64      `json:"owedSen"`
	Paid    bool       `json:"paid"`
	PaidAt  *time.Time `json:"paidAt"`
}

type CreateSplitInput struct {
	Merchant    string     `json:"merchant"`
	OwnerName   string     `json:"ownerName"`
	OwnerQRURL  *string    `json:"ownerQrUrl"`
	CapturedAt  *time.Time `json:"capturedAt"`
	SubtotalSen int64      `json:"subtotalSen"`
	SSTSen      int64      `json:"sstSen"`
	ServiceSen  int64      `json:"serviceSen"`
	RoundingSen int64      `json:"roundingSen"`
	TotalSen    int64      `json:"totalSen"`
	Items       []struct {
		Name            string `json:"name"`
		Qty             int    `json:"qty"`
		UnitPriceSen    int64  `json:"unitPriceSen"`
		LineTotalSen    int64  `json:"lineTotalSen"`
		IncludedInSplit *bool  `json:"includedInSplit"`
	} `json:"items"`
}

// ---- splits ----

func (st *Store) CreateSplit(ctx context.Context, ownerID, slug string, in CreateSplitInput) (Split, error) {
	tx, err := st.pool.Begin(ctx)
	if err != nil {
		return Split{}, err
	}
	defer tx.Rollback(ctx)

	var s Split
	err = tx.QueryRow(ctx, `
		insert into splits (owner_id, slug, merchant, owner_name, owner_qr_url, captured_at,
		                    subtotal_sen, sst_sen, service_sen, rounding_sen, total_sen)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		returning id, slug, merchant, owner_name, owner_qr_url, captured_at,
		          subtotal_sen, sst_sen, service_sen, rounding_sen, total_sen`,
		ownerID, slug, in.Merchant, in.OwnerName, in.OwnerQRURL, in.CapturedAt,
		in.SubtotalSen, in.SSTSen, in.ServiceSen, in.RoundingSen, in.TotalSen,
	).Scan(&s.ID, &s.Slug, &s.Merchant, &s.OwnerName, &s.OwnerQRURL, &s.CapturedAt,
		&s.SubtotalSen, &s.SSTSen, &s.ServiceSen, &s.RoundingSen, &s.TotalSen)
	if err != nil {
		return Split{}, err
	}
	s.OwnerID = ownerID

	for i, it := range in.Items {
		qty := it.Qty
		if qty < 1 {
			qty = 1
		}
		included := true
		if it.IncludedInSplit != nil {
			included = *it.IncludedInSplit
		}
		if _, err = tx.Exec(ctx, `
			insert into items (split_id, name, qty, unit_price_sen, line_total_sen, position, included_in_split)
			values ($1,$2,$3,$4,$5,$6,$7)`,
			s.ID, it.Name, qty, it.UnitPriceSen, it.LineTotalSen, i, included); err != nil {
			return Split{}, err
		}
	}

	// The owner is a participant too (their own share shows in tracking).
	if _, err = tx.Exec(ctx,
		`insert into participants (split_id, name, is_owner) values ($1,$2,true)`,
		s.ID, in.OwnerName); err != nil {
		return Split{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return Split{}, err
	}
	return s, nil
}

func (st *Store) GetSplitBySlug(ctx context.Context, slug string) (Split, error) {
	var s Split
	err := st.pool.QueryRow(ctx, `
		select id, slug, merchant, owner_id, owner_name, owner_qr_url, captured_at,
		       subtotal_sen, sst_sen, service_sen, rounding_sen, total_sen
		from splits where slug = $1`, slug,
	).Scan(&s.ID, &s.Slug, &s.Merchant, &s.OwnerID, &s.OwnerName, &s.OwnerQRURL, &s.CapturedAt,
		&s.SubtotalSen, &s.SSTSen, &s.ServiceSen, &s.RoundingSen, &s.TotalSen)
	if errors.Is(err, pgx.ErrNoRows) {
		return Split{}, errNotFound
	}
	return s, err
}

// ListOwnerSplits returns the owner's splits newest first, with collected totals.
func (st *Store) ListOwnerSplits(ctx context.Context, ownerID string) ([]SplitSummary, error) {
	rows, err := st.pool.Query(ctx, `
		select s.id, s.slug, s.merchant, s.owner_name, s.owner_qr_url, s.captured_at,
		       s.subtotal_sen, s.sst_sen, s.service_sen, s.rounding_sen, s.total_sen, s.created_at,
		       coalesce((select sum(owed_sen) from participants where split_id = s.id and paid and not is_owner), 0)
		from splits s
		where s.owner_id = $1
		order by s.created_at desc
		limit 50`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SplitSummary
	for rows.Next() {
		var sum SplitSummary
		if err := rows.Scan(
			&sum.Split.ID, &sum.Split.Slug, &sum.Split.Merchant, &sum.Split.OwnerName, &sum.Split.OwnerQRURL,
			&sum.Split.CapturedAt, &sum.Split.SubtotalSen, &sum.Split.SSTSen, &sum.Split.ServiceSen,
			&sum.Split.RoundingSen, &sum.Split.TotalSen, &sum.Split.CreatedAt, &sum.CollectedSen,
		); err != nil {
			return nil, err
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

func (st *Store) DeleteSplit(ctx context.Context, splitID, ownerID string) error {
	tag, err := st.pool.Exec(ctx, `delete from splits where id = $1 and owner_id = $2`, splitID, ownerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotFound
	}
	return nil
}

// ---- items ----

func (st *Store) ListItems(ctx context.Context, splitID string) ([]Item, error) {
	return st.listItems(ctx, splitID, false)
}

func (st *Store) ListSplittableItems(ctx context.Context, splitID string) ([]Item, error) {
	return st.listItems(ctx, splitID, true)
}

func (st *Store) listItems(ctx context.Context, splitID string, splittableOnly bool) ([]Item, error) {
	filter := ""
	if splittableOnly {
		filter = " and i.included_in_split = true"
	}
	rows, err := st.pool.Query(ctx, `
		select i.id, i.name, i.qty, i.unit_price_sen, i.line_total_sen, i.position, i.included_in_split,
		       coalesce(array_agg(p.name) filter (where p.name is not null), '{}') as claimed_by
		from items i
		left join claims c on c.item_id = i.id
		left join participants p on p.id = c.participant_id
		where i.split_id = $1`+filter+`
		group by i.id
		order by i.position`, splitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Name, &it.Qty, &it.UnitPriceSen, &it.LineTotalSen,
			&it.Position, &it.IncludedInSplit, &it.ClaimedBy); err != nil {
			return nil, err
		}
		it.Claimants = len(it.ClaimedBy)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (st *Store) SplittableSubtotalSen(ctx context.Context, splitID string) (int64, error) {
	var sum int64
	err := st.pool.QueryRow(ctx, `
		select coalesce(sum(line_total_sen), 0)
		from items where split_id = $1 and included_in_split = true`, splitID,
	).Scan(&sum)
	return sum, err
}

// ---- participants ----

func (st *Store) CreateParticipant(ctx context.Context, splitID, name, token string) (Participant, error) {
	var p Participant
	err := st.pool.QueryRow(ctx, `
		insert into participants (split_id, name, token) values ($1,$2,$3)
		returning id, name, is_owner, owed_sen, paid, paid_at`,
		splitID, name, token,
	).Scan(&p.ID, &p.Name, &p.IsOwner, &p.OwedSen, &p.Paid, &p.PaidAt)
	return p, err
}

// ParticipantByToken returns the participant and the split they belong to.
func (st *Store) ParticipantByToken(ctx context.Context, token string) (Participant, string, error) {
	var p Participant
	var splitID string
	err := st.pool.QueryRow(ctx, `
		select id, name, is_owner, owed_sen, paid, paid_at, split_id
		from participants where token = $1`, token,
	).Scan(&p.ID, &p.Name, &p.IsOwner, &p.OwedSen, &p.Paid, &p.PaidAt, &splitID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Participant{}, "", errNotFound
	}
	return p, splitID, err
}

// SetClaims replaces a participant's claimed items, then recomputes owed amounts
// for everyone in the split (one claim change shifts shared-item splits for all).
func (st *Store) SetClaims(ctx context.Context, splitID, participantID string, itemIDs []string) error {
	tx, err := st.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err = tx.Exec(ctx, `delete from claims where participant_id = $1`, participantID); err != nil {
		return err
	}
	for _, id := range itemIDs {
		// Guard: only items belonging to this split can be claimed.
		if _, err = tx.Exec(ctx, `
			insert into claims (participant_id, item_id)
			select $1, $2 where exists (
				select 1 from items where id = $2 and split_id = $3 and included_in_split = true
			)
			on conflict do nothing`, participantID, id, splitID); err != nil {
			return err
		}
	}
	if err = recomputeOwedTx(ctx, tx, splitID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// recomputeOwedTx recalculates owed_sen for every participant in the split.
func recomputeOwedTx(ctx context.Context, tx pgx.Tx, splitID string) error {
	var sst, svc, rnd int64
	if err := tx.QueryRow(ctx,
		`select sst_sen, service_sen, rounding_sen from splits where id = $1`, splitID,
	).Scan(&sst, &svc, &rnd); err != nil {
		return err
	}
	taxTotal := sst + svc + rnd

	var splittableSub int64
	if err := tx.QueryRow(ctx, `
		select coalesce(sum(line_total_sen), 0)
		from items where split_id = $1 and included_in_split = true`, splitID,
	).Scan(&splittableSub); err != nil {
		return err
	}

	lineTotal := map[string]int64{}
	claimants := map[string]int{}
	rows, err := tx.Query(ctx, `
		select id, line_total_sen from items
		where split_id = $1 and included_in_split = true`, splitID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		var lt int64
		if err := rows.Scan(&id, &lt); err != nil {
			rows.Close()
			return err
		}
		lineTotal[id] = lt
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	itemsByParticipant := map[string][]string{}
	crows, err := tx.Query(ctx, `
		select c.participant_id, c.item_id
		from claims c join items i on i.id = c.item_id
		where i.split_id = $1 and i.included_in_split = true`, splitID)
	if err != nil {
		return err
	}
	for crows.Next() {
		var pid, iid string
		if err := crows.Scan(&pid, &iid); err != nil {
			crows.Close()
			return err
		}
		itemsByParticipant[pid] = append(itemsByParticipant[pid], iid)
		claimants[iid]++
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		return err
	}

	for pid, itemIDs := range itemsByParticipant {
		items := make([]share.Item, 0, len(itemIDs))
		for _, iid := range itemIDs {
			items = append(items, share.Item{LineTotalSen: lineTotal[iid], Claimants: claimants[iid]})
		}
		owed := share.Owed(share.ClaimedSen(items), splittableSub, taxTotal)
		if _, err := tx.Exec(ctx, `update participants set owed_sen = $1 where id = $2`, owed, pid); err != nil {
			return err
		}
	}
	// Participants with no claims owe nothing.
	if _, err := tx.Exec(ctx, `
		update participants set owed_sen = 0
		where split_id = $1 and id not in (select distinct participant_id from claims)`, splitID); err != nil {
		return err
	}
	return nil
}

// MarkPaid flags a participant paid and returns the row plus its split id.
func (st *Store) MarkPaid(ctx context.Context, participantID string) (Participant, string, error) {
	var p Participant
	var splitID string
	err := st.pool.QueryRow(ctx, `
		update participants set paid = true, paid_at = now()
		where id = $1
		returning id, name, is_owner, owed_sen, paid, paid_at, split_id`, participantID,
	).Scan(&p.ID, &p.Name, &p.IsOwner, &p.OwedSen, &p.Paid, &p.PaidAt, &splitID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Participant{}, "", errNotFound
	}
	return p, splitID, err
}

// Participants lists everyone in a split, owner first then join order.
func (st *Store) Participants(ctx context.Context, splitID string) ([]Participant, error) {
	rows, err := st.pool.Query(ctx, `
		select id, name, is_owner, owed_sen, paid, paid_at
		from participants where split_id = $1
		order by is_owner desc, created_at`, splitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Participant
	for rows.Next() {
		var p Participant
		if err := rows.Scan(&p.ID, &p.Name, &p.IsOwner, &p.OwedSen, &p.Paid, &p.PaidAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

type OwnerProfile struct {
	OwnerQRURL     *string `json:"ownerQrUrl"`
	AutoFillAmount bool    `json:"autoFillAmount"`
}

func (st *Store) GetOwnerProfile(ctx context.Context, ownerID string) (OwnerProfile, error) {
	var p OwnerProfile
	err := st.pool.QueryRow(ctx, `
		select owner_qr_url, auto_fill_amount
		from owner_profiles where owner_id = $1`, ownerID,
	).Scan(&p.OwnerQRURL, &p.AutoFillAmount)
	if err == nil {
		return p, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return OwnerProfile{AutoFillAmount: true}, nil
	}
	// Table not migrated yet — return defaults so the settings UI still loads.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return OwnerProfile{AutoFillAmount: true}, nil
	}
	return OwnerProfile{}, err
}

func (st *Store) SetOwnerQRURL(ctx context.Context, ownerID, qrURL string) (OwnerProfile, error) {
	var p OwnerProfile
	err := st.pool.QueryRow(ctx, `
		insert into owner_profiles (owner_id, owner_qr_url)
		values ($1, $2)
		on conflict (owner_id) do update set
			owner_qr_url = excluded.owner_qr_url,
			updated_at = now()
		returning owner_qr_url, auto_fill_amount`, ownerID, qrURL,
	).Scan(&p.OwnerQRURL, &p.AutoFillAmount)
	return p, err
}

func (st *Store) SetAutoFillAmount(ctx context.Context, ownerID string, autoFill bool) (OwnerProfile, error) {
	var p OwnerProfile
	err := st.pool.QueryRow(ctx, `
		insert into owner_profiles (owner_id, auto_fill_amount)
		values ($1, $2)
		on conflict (owner_id) do update set
			auto_fill_amount = excluded.auto_fill_amount,
			updated_at = now()
		returning owner_qr_url, auto_fill_amount`, ownerID, autoFill,
	).Scan(&p.OwnerQRURL, &p.AutoFillAmount)
	return p, err
}

// CollectedSen sums what paid friends have settled.
func (st *Store) CollectedSen(ctx context.Context, splitID string) (int64, error) {
	var sum int64
	err := st.pool.QueryRow(ctx,
		`select coalesce(sum(owed_sen),0) from participants where split_id = $1 and paid and not is_owner`, splitID,
	).Scan(&sum)
	return sum, err
}

// FriendsExpectedSen is the total owed by non-owner participants.
func (st *Store) FriendsExpectedSen(ctx context.Context, splitID string) (int64, error) {
	var sum int64
	err := st.pool.QueryRow(ctx,
		`select coalesce(sum(owed_sen),0) from participants where split_id = $1 and not is_owner`, splitID,
	).Scan(&sum)
	return sum, err
}
