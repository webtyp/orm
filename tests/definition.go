package tests

import (
	"webtyp.com/model"
)

var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "first_name", Type: model.Text(), NotNull: true},
		{Name: "last_name", Type: model.Text()},
		{Name: "email", Type: model.Text(), DB: &model.FieldDB{Unique: true}},
		{Name: "score", Type: model.Float()},
		{Name: "is_active", Type: model.Bool()},
		{Name: "avatar", Type: model.Blob()},
	},
}

var OrderModel = model.Definition{
	Name: "order",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "user_id", Type: model.Int(), Ref: &UserModel, DB: &model.FieldDB{RefColumn: "id"}},
		{Name: "total", Type: model.Float()},
	},
}

var ModelWithIgnoredModel = model.Definition{
	Name: "model_with_ignored",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
		{Name: "tags", Type: model.Text(), Exclude: true}, // Exclude replaces the need for db:"-" for in-memory fields
		{Name: "score", Type: model.Float()},
	},
}

var MultiAModel = model.Definition{
	Name: "multi_a_records",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
	},
}

var MultiBModel = model.Definition{
	Name: "multi_b",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "value", Type: model.Int()},
	},
}

var NumericTypesModel = model.Definition{
	Name: "numeric_types",
	Fields: model.Fields{
		{Name: "id_numeric", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		{Name: "count_uint", Type: model.Int()},
		{Name: "ratio_f32", Type: model.Float()},
	},
}

var RefNoColumnModel = model.Definition{
	Name: "ref_no_column",
	Fields: model.Fields{
		{Name: "id_ref", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "parent_id", Type: model.Int(), Ref: &MultiAModel},
	},
}

var PointerReceiverModel = model.Definition{
	Name: "ptr_table",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
	},
}

var UserFormModel = model.Definition{
	Name: "user_form",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text(), Permitted: model.Permitted{Minimum: 2, Maximum: 100}},
		{Name: "email", Type: model.Text(), NotNull: true},
		{Name: "password", Type: model.Text(), NotNull: true, Permitted: model.Permitted{Minimum: 8}},
		{Name: "bio", Type: model.Text(), Permitted: model.Permitted{Tilde: true, Spaces: true}},
		{Name: "age", Type: model.Int()},
	},
}

var LoginFormModel = model.Definition{
	Name: "login_form",
	Fields: model.Fields{
		{Name: "email", Type: model.Text(), NotNull: true},
		{Name: "password", Type: model.Text(), NotNull: true},
	},
}

var AddressModel = model.Definition{
	Name: "address",
	Fields: model.Fields{
		{Name: "street", Type: model.Text()},
		{Name: "city", Type: model.Text()},
	},
}

var UserWithCompositionModel = model.Definition{
	Name: "user_with_composition",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: model.Text()},
		{Name: "home_addr", Type: model.Struct(&AddressModel)},
	},
}

var UserWithNoTildeModel = model.Definition{
	Name: "user_with_no_tilde",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "nombre", Type: model.Text(), NotNull: true},
	},
}

var ShortAutoIncModel = model.Definition{
	Name: "short_auto_inc",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true, AutoInc: true}},
		{Name: "value", Type: model.Int()},
	},
}

var OmitEmptyDemoModel = model.Definition{
	Name: "omit_empty_demo",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "email", Type: model.Text(), OmitEmpty: true, DB: &model.FieldDB{Unique: true}},
		{Name: "nickname", Type: model.Text()},
	},
}
