# --- Kataloge (Stammdaten) ---------------------------------------------------

table "partner__address_roles" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  column "is_main" {
    type    = boolean
    default = false
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.code] }
}

table "partner__comm_categories" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  primary_key { columns = [column.code] }
}

table "partner__comm_types" {
  schema = schema.main
  column "code"          { type = text }
  column "category_code" { type = text }
  column "description"   { type = text }
  column "is_main" {
    type    = boolean
    default = false
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.code] }
  foreign_key "partner__comm_types_category" {
    columns     = [column.category_code]
    ref_columns = [table.partner__comm_categories.column.code]
  }
}

table "partner__role_types" {
  schema = schema.main
  column "code"        { type = text }
  column "description" { type = text }
  column "is_debitor" {
    type    = boolean
    default = false
  }
  column "is_creditor" {
    type    = boolean
    default = false
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.code] }
}

# --- Geschäftspartner und Beziehungen -----------------------------------------

table "partner__bp" {
  schema = schema.main
  column "id"   { type = text }
  column "type" { type = text }
  column "name1" { type = text }
  column "name2" {
    type = text
    null = true
  }
  column "search_term" {
    type = text
    null = true
  }
  column "is_blocked" {
    type    = boolean
    default = false
  }
  # Status-Flag (Lebenszyklus status): false = inaktiviert. Seit 0.4.0.
  column "is_active" {
    type    = boolean
    default = true
  }
  primary_key { columns = [column.id] }
  index "partner__bp_search" { columns = [column.search_term] }
}

table "partner__roles" {
  schema = schema.main
  column "bp_id"      { type = text }
  column "role_code"  { type = text }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  primary_key { columns = [column.bp_id, column.role_code, column.valid_from] }
  foreign_key "partner__roles_bp" {
    columns     = [column.bp_id]
    ref_columns = [table.partner__bp.column.id]
  }
  foreign_key "partner__roles_role" {
    columns     = [column.role_code]
    ref_columns = [table.partner__role_types.column.code]
  }
}

table "partner__addresses" {
  schema = schema.main
  column "id"     { type = text }
  column "street" { type = text }
  column "house_no" {
    type = text
    null = true
  }
  column "zip_code" { type = text }
  column "city"     { type = text }
  column "country"  { type = text }
  primary_key { columns = [column.id] }
}

table "partner__bp_addresses" {
  schema = schema.main
  column "id"                { type = text }
  column "bp_id"             { type = text }
  column "address_id"        { type = text }
  column "address_role_code" { type = text }
  column "valid_from"        { type = date }
  column "valid_to"          { type = date }
  column "is_default" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.id] }
  index "partner__bp_addresses_bp" { columns = [column.bp_id] }
  foreign_key "partner__bp_addresses_bp" {
    columns     = [column.bp_id]
    ref_columns = [table.partner__bp.column.id]
  }
  foreign_key "partner__bp_addresses_address" {
    columns     = [column.address_id]
    ref_columns = [table.partner__addresses.column.id]
  }
  foreign_key "partner__bp_addresses_role" {
    columns     = [column.address_role_code]
    ref_columns = [table.partner__address_roles.column.code]
  }
}

table "partner__contacts" {
  schema = schema.main
  column "id"             { type = text }
  column "bp_id"          { type = text }
  column "comm_type_code" { type = text }
  column "value"          { type = text }
  column "valid_from"     { type = date }
  column "valid_to"       { type = date }
  column "is_default" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.id] }
  index "partner__contacts_bp" { columns = [column.bp_id] }
  foreign_key "partner__contacts_bp" {
    columns     = [column.bp_id]
    ref_columns = [table.partner__bp.column.id]
  }
  foreign_key "partner__contacts_type" {
    columns     = [column.comm_type_code]
    ref_columns = [table.partner__comm_types.column.code]
  }
}

table "partner__bank_details" {
  schema = schema.main
  column "id"    { type = text }
  column "bp_id" { type = text }
  column "iban"  { type = text }
  column "bic" {
    type = text
    null = true
  }
  column "bank_name" {
    type = text
    null = true
  }
  column "account_holder" {
    type = text
    null = true
  }
  column "valid_from" { type = date }
  column "valid_to"   { type = date }
  column "is_default" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.id] }
  index "partner__bank_details_bp" { columns = [column.bp_id] }
  foreign_key "partner__bank_details_bp" {
    columns     = [column.bp_id]
    ref_columns = [table.partner__bp.column.id]
  }
}

table "partner__company_codes" {
  schema = schema.main
  column "bp_id"        { type = text }
  column "company_code" { type = text }
  column "role_code"    { type = text }
  column "reconciliation_account" {
    type = text
    null = true
  }
  column "payment_terms" {
    type = text
    null = true
  }
  column "dunning_block" {
    type    = boolean
    default = false
  }
  column "posting_block" {
    type    = boolean
    default = false
  }
  primary_key { columns = [column.bp_id, column.company_code, column.role_code] }
  foreign_key "partner__company_codes_bp" {
    columns     = [column.bp_id]
    ref_columns = [table.partner__bp.column.id]
  }
  foreign_key "partner__company_codes_role" {
    columns     = [column.role_code]
    ref_columns = [table.partner__role_types.column.code]
  }
}
