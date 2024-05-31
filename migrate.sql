--Create companies Table
			CREATE TABLE companies (
				id char(36) NOT NULL PRIMARY KEY,
				name varchar(100) NOT NULL,
				code char(4) NOT NULL UNIQUE,
				address varchar NOT NULL,
				city varchar(100) NOT NULL,
				province varchar(100) NOT NULL,
				npwp char(20) NULL,
				phone varchar(20) NOT NULL,
				pic varchar(100) NOT NULL,
				pic_phone varchar(20) NOT NULL,
				logo varchar NULL,
				package_of_feature_id char(36) NOT NULL,
				created_at timestamp NOT NULL DEFAULT NOW(),
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NULL 
			);
		
--Create regions Table
			CREATE TABLE regions (
				id char(36) NOT NULL PRIMARY KEY,
				company_id char(36) NOT NULL,
				name varchar(100) NOT NULL,
				code char(4) NOT NULL,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				UNIQUE(company_id, code),
				CONSTRAINT fk_regions_to_companies FOREIGN KEY(company_id) REFERENCES companies(id)
			);
		
--Create branches Table
			CREATE TABLE branches (
				id char(36) NOT NULL PRIMARY KEY,
				company_id char(36) NOT NULL,
				name varchar(100) NOT NULL,
				code char(4) NOT NULL,
				address varchar NOT NULL,
				city varchar(100) NOT NULL,
				province varchar(100) NOT NULL,
				npwp char(20) NULL,
				phone varchar(20) NOT NULL,
				pic varchar(100) NOT NULL,
				pic_phone varchar(20) NOT NULL,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				UNIQUE(company_id, code),
				CONSTRAINT fk_branches_to_companies FOREIGN KEY(company_id) REFERENCES companies(id)
			);
		
--Create branches_regions Table
			CREATE TABLE branches_regions (
				id char(36) NOT NULL PRIMARY KEY,
				region_id char(36) NOT NULL,
				branch_id char(36) NOT NULL,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				UNIQUE(region_id, branch_id),
				CONSTRAINT fk_branches_regions_to_regions FOREIGN KEY(region_id) REFERENCES regions(id) ON DELETE CASCADE,
				CONSTRAINT fk_branches_regions_to_branches FOREIGN KEY(branch_id) REFERENCES branches(id) ON DELETE CASCADE
			);
		
--Create groups Table
			CREATE TABLE groups (
				id char(36) NOT NULL PRIMARY KEY,
				company_id char(36) NOT NULL,
				name varchar(100) NOT NULL,
				is_mutable bool NOT NULL DEFAULT false,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				CONSTRAINT fk_groups_to_companies FOREIGN KEY(company_id) REFERENCES companies(id)
			);
		
--Create users Table
			CREATE TABLE users (
				id char(36) NOT NULL PRIMARY KEY,
				company_id char(36) NOT NULL,
				region_id char(36) NULL,
				branch_id char(36) NULL,
				group_id char(36) NOT NULL,
				username varchar(20) NOT NULL UNIQUE,
				password varchar NOT NULL,
				name varchar(100) NOT NULL,
				email varchar(100) NOT NULL UNIQUE,
				created_at timestamp NOT NULL DEFAULT NOW(),
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NULL,
				CONSTRAINT fk_users_to_companies FOREIGN KEY(company_id) REFERENCES companies(id),
				CONSTRAINT fk_users_to_regions FOREIGN KEY(region_id) REFERENCES regions(id),
				CONSTRAINT fk_users_to_branches FOREIGN KEY(branch_id) REFERENCES branches(id),
				CONSTRAINT fk_users_to_groups FOREIGN KEY(group_id) REFERENCES groups(id)
			);
		
--Create access Table
			CREATE TABLE access (
				id char(36) NOT NULL PRIMARY KEY,
				parent_id char(36) NULL,
				name varchar(100) NOT NULL UNIQUE,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				CONSTRAINT fk_access_to_parents FOREIGN KEY(parent_id) REFERENCES access(id) 
			);
		
--Create access_groups Table
			CREATE TABLE access_groups (
				id char(36) NOT NULL PRIMARY KEY,
				group_id char(36) NOT NULL,
				access_id char(36) NOT NULL,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				UNIQUE(group_id, access_id),
				CONSTRAINT fk_access_groups_to_groups FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE,
				CONSTRAINT fk_access_groups_to_access FOREIGN KEY(access_id) REFERENCES access(id) ON DELETE CASCADE
			);
		
--Create employees Table
			CREATE TABLE employees (
				id char(36) NOT NULL PRIMARY KEY,
				user_id char(36) NOT NULL UNIQUE,
				name varchar(100) NOT NULL,
				code char(20) NOT NULL UNIQUE,
				address varchar NOT NULL,
				city varchar(100) NOT NULL,
				province varchar(100) NOT NULL,
				jabatan varchar(100) NOT NULL,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				CONSTRAINT fk_employees_to_users FOREIGN KEY(user_id) REFERENCES users(id)
			);
		
--Create package_features Table
			CREATE TABLE package_features (
				id char(36) NOT NULL PRIMARY KEY,
				name varchar(100) NOT NULL UNIQUE,
				created_at timestamp NOT NULL DEFAULT NOW(),
				updated_at timestamp NOT NULL DEFAULT NOW()
			);
		
--Create features Table
			CREATE TABLE features (
				id char(36) NOT NULL PRIMARY KEY,
				name varchar(100) NOT NULL UNIQUE,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL
			);
		
--Create features_package_features Table
			CREATE TABLE features_package_features (
				id char(36) NOT NULL PRIMARY KEY,
				package_feature_id char(36) NOT NULL,
				feature_id char(36) NOT NULL,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				UNIQUE(package_feature_id, feature_id),
				CONSTRAINT fk_features_package_features_to_package_features FOREIGN KEY(package_feature_id) REFERENCES package_features(id) ON DELETE CASCADE,
				CONSTRAINT fk_features_package_features_to_features FOREIGN KEY(feature_id) REFERENCES features(id) ON DELETE CASCADE
			);
		
--Create companies_features Table
			CREATE TABLE companies_features (
				id char(36) NOT NULL PRIMARY KEY,
				company_id char(36) NOT NULL,
				feature_id char(36) NOT NULL,
				created_at timestamp NOT NULL DEFAULT NOW(),
				created_by char(36) NOT NULL,
				updated_at timestamp NOT NULL DEFAULT NOW(),
				updated_by char(36) NOT NULL,
				UNIQUE(company_id, feature_id),
				CONSTRAINT fk_companies_features_to_companies FOREIGN KEY(company_id) REFERENCES companies(id) ON DELETE CASCADE,
				CONSTRAINT fk_companies_features_to_features FOREIGN KEY(feature_id) REFERENCES features(id) ON DELETE CASCADE
			);
		
--Create request_passwords Table
			CREATE TABLE request_passwords (
				id char(36) NOT NULL PRIMARY KEY,
				user_id char(36) NOT NULL,
				is_used boolean NOT NULL DEFAULT false,
				created_at timestamp NOT NULL DEFAULT NOW(),
				CONSTRAINT fk_request_passwords_to_users FOREIGN KEY(user_id) REFERENCES users(id)
			);
		
