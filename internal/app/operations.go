package app

import (
	"strconv"

	"github.com/hilather/go-lab-ntp/internal/domainerr"
	"github.com/hilather/go-lab-ntp/internal/model"
)

func applyOperations(st *model.State, ops []model.Operation) error {
	if st == nil {
		return domainerr.ValidationFailed("nil state",
			domainerr.FieldViolation{Path: "", Code: "required", Message: "state is nil"})
	}
	for i, op := range ops {
		if err := applyOne(st, op, i); err != nil {
			return err
		}
	}
	return nil
}

func applyOne(st *model.State, op model.Operation, i int) error {
	path := "operations[" + strconv.Itoa(i) + "]"
	if !model.KnownOp(op.Op) {
		return domainerr.ValidationFailed("unknown operation",
			domainerr.FieldViolation{Path: path + ".op", Code: "invalid_value", Message: "unknown op; listen/NTS/keys/auth are reset-only"}).
			WithRemediation("reset-only; rewrite bootstrap and POST /v1/state:reset")
	}
	switch op.Op {
	case model.OpReplaceFilters:
		st.Spec.Filters = append([]model.Filter(nil), op.Filters...)
	case model.OpUpsertFilter:
		if op.Filter == nil {
			return domainerr.ValidationFailed("missing filter",
				domainerr.FieldViolation{Path: path + ".filter", Code: "required", Message: "upsertFilter requires filter"})
		}
		upsertFilter(&st.Spec.Filters, *op.Filter)
	case model.OpRemoveFilter:
		if op.Name == "" {
			return domainerr.ValidationFailed("missing name",
				domainerr.FieldViolation{Path: path + ".name", Code: "required", Message: "removeFilter requires name"})
		}
		if !removeFilter(&st.Spec.Filters, op.Name) {
			return domainerr.NotFound("filter " + op.Name + " not found")
		}
	case model.OpReplaceRestrict:
		if op.Restrict == nil {
			return domainerr.ValidationFailed("missing restrict",
				domainerr.FieldViolation{Path: path + ".restrict", Code: "required", Message: "replaceRestrict requires restrict"})
		}
		st.Spec.NTP.Restrict = *op.Restrict
	case model.OpReplaceAdmission:
		if op.Admission == nil {
			return domainerr.ValidationFailed("missing admission",
				domainerr.FieldViolation{Path: path + ".admission", Code: "required", Message: "replaceAdmission requires admission"})
		}
		st.Spec.NTP.Admission = *op.Admission
	case model.OpReplaceAllowClientCidrs:
		if op.AllowClientCidrs == nil {
			st.Spec.NTP.AllowClientCidrs = nil
		} else {
			st.Spec.NTP.AllowClientCidrs = append([]string{}, op.AllowClientCidrs...)
		}
	case model.OpReplaceQueryLog:
		if op.QueryLog == nil {
			return domainerr.ValidationFailed("missing queryLog",
				domainerr.FieldViolation{Path: path + ".queryLog", Code: "required", Message: "replaceQueryLog requires queryLog"})
		}
		st.Spec.NTP.QueryLog = *op.QueryLog
	case model.OpReplaceManagementHTTP:
		if op.ManagementHTTP == nil {
			return domainerr.ValidationFailed("missing managementHTTP",
				domainerr.FieldViolation{Path: path + ".managementHTTP", Code: "required", Message: "replaceManagementHTTP requires managementHTTP"})
		}
		st.Spec.Management.BodyLimit = op.ManagementHTTP.BodyLimit
		st.Spec.Management.RequestsPerSecond = op.ManagementHTTP.RequestsPerSecond
		st.Spec.Management.Burst = op.ManagementHTTP.Burst
		st.Spec.Management.MaxConcurrent = op.ManagementHTTP.MaxConcurrent
	}
	return nil
}

func upsertFilter(list *[]model.Filter, f model.Filter) {
	for i := range *list {
		if (*list)[i].Name == f.Name {
			(*list)[i] = f
			return
		}
	}
	*list = append(*list, f)
}

func removeFilter(list *[]model.Filter, name string) bool {
	dst := (*list)[:0]
	found := false
	for _, f := range *list {
		if f.Name == name {
			found = true
			continue
		}
		dst = append(dst, f)
	}
	*list = dst
	return found
}

func rejectResetOnly(before, after *model.State) error {
	if before == nil || after == nil {
		return nil
	}
	rem := "reset-only; rewrite bootstrap and POST /v1/state:reset"
	if before.Spec.Listeners != after.Spec.Listeners {
		return domainerr.ValidationFailed("listeners are reset-only",
			domainerr.FieldViolation{Path: "spec.listeners", Code: "invalid_value", Message: "listen addresses cannot change via Apply"}).
			WithRemediation(rem)
	}
	if before.Spec.NTP.NTS != after.Spec.NTP.NTS {
		return domainerr.ValidationFailed("ntp.nts is reset-only",
			domainerr.FieldViolation{Path: "spec.ntp.nts", Code: "invalid_value", Message: "NTS cannot change via Apply"}).
			WithRemediation(rem)
	}
	if before.Spec.NTP.SymmetricKeys != after.Spec.NTP.SymmetricKeys {
		return domainerr.ValidationFailed("ntp.symmetricKeys is reset-only",
			domainerr.FieldViolation{Path: "spec.ntp.symmetricKeys", Code: "invalid_value", Message: "keys file cannot change via Apply"}).
			WithRemediation(rem)
	}
	if !authEqual(before.Spec.Auth, after.Spec.Auth) {
		return domainerr.ValidationFailed("spec.auth is reset-only",
			domainerr.FieldViolation{Path: "spec.auth", Code: "invalid_value", Message: "auth cannot change via Apply"}).
			WithRemediation(rem)
	}
	return nil
}

func authEqual(a, b model.AuthSpec) bool {
	if a.Mode != b.Mode || len(a.Tokens) != len(b.Tokens) {
		return false
	}
	for i := range a.Tokens {
		if a.Tokens[i].ID != b.Tokens[i].ID || a.Tokens[i].Role != b.Tokens[i].Role || a.Tokens[i].SecretFile != b.Tokens[i].SecretFile {
			return false
		}
	}
	return true
}
