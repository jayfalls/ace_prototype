// Test file to check enum extraction edge cases
package annot8

// EnumWithImplicitType tests const with implicit type from predecessor
type StatusEnum string

const (
	StatusActive   StatusEnum = "active"
	StatusInactive            = "inactive" // Implicitly StatusEnum
	StatusPending  StatusEnum = "pending"
)

// EnumWithMixedDeclarations tests multiple types in const block
type TypeA string
type TypeB string

const (
	TypeAVal1 TypeA = "a1"
	TypeAVal2 TypeA = "a2"
	TypeBVal1 TypeB = "b1"
	TypeBVal2 TypeB = "b2"
)

// EnumFromOtherPackage simulates how sqlc enum constants might look
// (Note: This would be in pkg/db/sqlc package in real code)
// type DiscountType string
// const (
//     DiscountTypePercentage DiscountType = "percentage"
//     DiscountTypeFixed      DiscountType = "fixed"
// )
