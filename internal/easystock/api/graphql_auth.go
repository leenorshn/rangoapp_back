package api

import (
	"context"

	"rangoapp/internal/easystock/authn"
	"rangoapp/internal/easystock/billing"
	"rangoapp/internal/easystock/domain"
	"rangoapp/internal/easystock/tenant"

	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
	"github.com/shopspring/decimal"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
)

func authAPI(deps Deps) (graphql.Fields, graphql.Fields) {
	sessionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AuthSession",
		Fields: graphql.Fields{
			"accessToken":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"refreshToken":       &graphql.Field{Type: graphql.String},
			"expiresAt":          &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"sessionId":          &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"userId":             &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"companyId":          &graphql.Field{Type: graphql.ID},
			"role":               &graphql.Field{Type: graphql.String},
			"scope":              &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"mustChangePassword": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"csrfToken":          &graphql.Field{Type: graphql.String},
		},
	})
	meType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Me",
		Fields: graphql.Fields{
			"id":                 &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"fullName":           &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"phone":              &graphql.Field{Type: graphql.String},
			"phoneVerified":      &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"email":              &graphql.Field{Type: graphql.String},
			"mustChangePassword": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"companies": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.NewObject(graphql.ObjectConfig{
				Name: "MyCompany",
				Fields: graphql.Fields{
					"companyId":   &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
					"companyName": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
					"role":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
					"status":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
					"storeIds":    &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.ID)))},
				},
			}))))},
		},
	})
	deviceType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DeviceSession",
		Fields: graphql.Fields{
			"id":            &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"companyId":     &graphql.Field{Type: graphql.ID},
			"scope":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"createdAt":     &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"lastRefreshAt": &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"expiresAt":     &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"current":       &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"userAgent":     &graphql.Field{Type: graphql.String},
		},
	})
	employeeType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Employee",
		Fields: graphql.Fields{
			"userId":             &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"fullName":           &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"phone":              &graphql.Field{Type: graphql.String},
			"role":               &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"status":             &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"mustChangePassword": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"phoneVerified":      &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"storeIds":           &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.ID)))},
		},
	})
	subType := graphql.NewObject(graphql.ObjectConfig{
		Name: "OrganizationSubscription",
		Fields: graphql.Fields{
			"companyId":           &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"companyName":         &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"role":                &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"lifecycle":           &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"status":              &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"planCode":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"planName":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"amount":              &graphql.Field{Type: graphql.String},
			"currency":            &graphql.Field{Type: graphql.String},
			"durationMonths":      &graphql.Field{Type: graphql.Int},
			"expiresAt":           &graphql.Field{Type: graphql.DateTime},
			"timezone":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"message":             &graphql.Field{Type: graphql.String},
			"banner":              &graphql.Field{Type: graphql.String},
			"canDeclarePayment":   &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"renewalReference":    &graphql.Field{Type: graphql.String},
			"operatorName":        &graphql.Field{Type: graphql.String},
			"destinationAccount":  &graphql.Field{Type: graphql.String},
			"beneficiaryName":     &graphql.Field{Type: graphql.String},
			"paymentInstructions": &graphql.Field{Type: graphql.String},
			"paymentStatus":       &graphql.Field{Type: graphql.String},
			"paymentReason":       &graphql.Field{Type: graphql.String},
			"newExpiresAt":        &graphql.Field{Type: graphql.DateTime},
		},
	})
	paymentType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SubscriptionPayment",
		Fields: graphql.Fields{
			"id":                &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"status":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"expectedAmount":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"declaredAmount":    &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"currency":          &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"externalReference": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"decisionReason":    &graphql.Field{Type: graphql.String},
			"createdAt":         &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"newExpiresAt":      &graphql.Field{Type: graphql.DateTime},
		},
	})
	noticeType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AppNotification",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
			"kind":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"body":      &graphql.Field{Type: graphql.String},
			"createdAt": &graphql.Field{Type: graphql.NewNonNull(graphql.DateTime)},
			"readAt":    &graphql.Field{Type: graphql.DateTime},
		},
	})
	queries := graphql.Fields{
		"me": &graphql.Field{Type: graphql.NewNonNull(meType), Resolve: func(p graphql.ResolveParams) (any, error) {
			member, ok := sessionUser(p.Context)
			if !ok || deps.Auth == nil {
				return nil, errUnavailable("Authentification requise.")
			}
			profile, err := deps.Auth.Me(p.Context, member.UserID)
			if err != nil {
				return nil, err
			}
			companies := make([]map[string]any, 0, len(profile.Companies))
			for _, company := range profile.Companies {
				companies = append(companies, map[string]any{
					"companyId": company.CompanyID.String(), "companyName": company.CompanyName,
					"role": company.Role, "status": company.Status, "storeIds": idList(company.StoreIDs),
				})
			}
			return map[string]any{
				"id": profile.ID.String(), "fullName": profile.FullName, "phone": profile.Phone,
				"phoneVerified": profile.PhoneVerified, "email": profile.Email,
				"mustChangePassword": profile.MustChangePassword, "companies": companies,
			}, nil
		}},
		"mySessions": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(deviceType))), Resolve: func(p graphql.ResolveParams) (any, error) {
			member, err := requireUser(p.Context)
			if err != nil || deps.Auth == nil {
				if err == nil {
					return nil, errUnavailable("Authentification requise.")
				}
				return nil, err
			}
			rows, err := deps.Auth.Sessions(p.Context, member.UserID, member.SessionID)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				item := map[string]any{
					"id": row.ID.String(), "scope": row.Scope, "createdAt": row.CreatedAt,
					"lastRefreshAt": row.LastRefreshAt, "expiresAt": row.ExpiresAt,
					"current": row.Current, "userAgent": row.UserAgent,
				}
				if row.CompanyID != uuid.Nil {
					item["companyId"] = row.CompanyID.String()
				}
				out = append(out, item)
			}
			return out, nil
		}},
		"employees": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(employeeType))), Resolve: func(p graphql.ResolveParams) (any, error) {
			member, err := requireCompany(p.Context)
			if err != nil || deps.Auth == nil {
				if deps.Auth == nil && err == nil {
					return nil, errUnavailable("Authentification requise.")
				}
				return nil, err
			}
			rows, err := deps.Auth.Employees(p.Context, member)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				out = append(out, employeeMap(row))
			}
			return out, nil
		}},
		"myOrganizationSubscription": &graphql.Field{Type: graphql.NewNonNull(subType), Resolve: func(p graphql.ResolveParams) (any, error) {
			member, ok := sessionUser(p.Context)
			if !ok || deps.Billing == nil {
				return nil, errUnavailable("Authentification requise.")
			}
			if err := rejectRestricted(member); err != nil {
				return nil, err
			}
			screen, err := deps.Billing.Screen(p.Context, member)
			if err != nil {
				return nil, err
			}
			return screenMap(screen), nil
		}},
		"renewalInstructions": &graphql.Field{Type: graphql.NewNonNull(subType), Resolve: func(p graphql.ResolveParams) (any, error) {
			member, ok := sessionUser(p.Context)
			if !ok || deps.Billing == nil {
				return nil, errUnavailable("Authentification requise.")
			}
			if err := rejectRestricted(member); err != nil {
				return nil, err
			}
			screen, err := deps.Billing.Screen(p.Context, member)
			if err != nil {
				return nil, err
			}
			return screenMap(screen), nil
		}},
		"mySubscriptionPaymentRequests": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(paymentType))), Resolve: func(p graphql.ResolveParams) (any, error) {
			member, err := requireCompany(p.Context)
			if err != nil || deps.Billing == nil || member.OrgRole != "owner" {
				if err != nil {
					return nil, err
				}
				return nil, domain.PermissionDenied("billing", "Seul le propriétaire suit les paiements.")
			}
			rows, err := deps.Billing.ListPayments(p.Context, member.CompanyID)
			if err != nil {
				return nil, err
			}
			return paymentsMap(rows), nil
		}},
		"pendingSubscriptionPayments": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(paymentType))), Resolve: func(p graphql.ResolveParams) (any, error) {
			member, err := requireUser(p.Context)
			if err != nil || deps.Billing == nil {
				if err == nil {
					return nil, errUnavailable("Authentification requise.")
				}
				return nil, err
			}
			rows, err := deps.Billing.Pending(p.Context, member)
			if err != nil {
				return nil, err
			}
			return paymentsMap(rows), nil
		}},
		"myNotifications": &graphql.Field{Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(noticeType))), Resolve: func(p graphql.ResolveParams) (any, error) {
			member, err := requireUser(p.Context)
			if err != nil || deps.Billing == nil {
				if err == nil {
					return nil, errUnavailable("Authentification requise.")
				}
				return nil, err
			}
			rows, err := deps.Billing.Notifications(p.Context, member.UserID)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				item := map[string]any{"id": row.ID.String(), "kind": row.Kind, "body": row.Body, "createdAt": row.CreatedAt}
				if row.ReadAt != nil {
					item["readAt"] = *row.ReadAt
				}
				out = append(out, item)
			}
			return out, nil
		}},
	}
	mutations := graphql.Fields{
		"requestOwnerSignupOtp":   otpRequest(deps, authn.PurposeSignup),
		"requestPasswordResetOtp": otpRequest(deps, authn.PurposeReset),
		"verifyOwnerSignupOtp":    otpVerify(deps, authn.PurposeSignup),
		"verifyPasswordResetOtp":  otpVerify(deps, authn.PurposeReset),
		"completeOwnerSignup": &graphql.Field{
			Type: graphql.NewNonNull(sessionType),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(signupInput())}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if deps.Auth == nil {
					return nil, errUnavailable("Authentification indisponible.")
				}
				in := inputMap(p.Args["input"])
				result, err := deps.Auth.CompleteSignup(p.Context, authn.SignupInput{
					Grant: str(in["grant"]), Phone: str(in["phone"]), FullName: str(in["fullName"]),
					Password: str(in["password"]), Email: str(in["email"]), CompanyName: str(in["companyName"]),
					PrimaryCurrency: str(in["primaryCurrency"]), Timezone: str(in["timezone"]), Request: requestFrom(p.Context),
				})
				if err != nil {
					return nil, err
				}
				return sessionMap(p.Context, result, str(in["client"])), nil
			},
		},
		"login": &graphql.Field{
			Type: graphql.NewNonNull(sessionType),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(loginInput())}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if deps.Auth == nil {
					return nil, errUnavailable("Authentification indisponible.")
				}
				in := inputMap(p.Args["input"])
				companyID, err := argUUID(in["companyId"])
				if err != nil {
					return nil, err
				}
				result, err := deps.Auth.Login(p.Context, authn.LoginInput{
					Phone: str(in["phone"]), Password: str(in["password"]), CompanyID: companyID, Request: requestFrom(p.Context),
				})
				if err != nil {
					return nil, err
				}
				return sessionMap(p.Context, result, str(in["client"])), nil
			},
		},
		"refreshSession": &graphql.Field{
			Type: graphql.NewNonNull(sessionType),
			Args: graphql.FieldConfigArgument{"refreshToken": &graphql.ArgumentConfig{Type: graphql.String}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if deps.Auth == nil {
					return nil, errUnavailable("Authentification indisponible.")
				}
				token := str(p.Args["refreshToken"])
				client := "mobile"
				if token == "" {
					token = refreshCookie(p.Context)
					client = "web"
					if err := checkCSRF(p.Context); err != nil {
						return nil, err
					}
				}
				result, err := deps.Auth.Refresh(p.Context, token, requestFrom(p.Context))
				if err != nil {
					return nil, err
				}
				return sessionMap(p.Context, result, client), nil
			},
		},
		"logout": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: func(p graphql.ResolveParams) (any, error) {
			return logoutOne(p, deps)
		}},
		"logoutAllSessions": &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean), Resolve: func(p graphql.ResolveParams) (any, error) {
			if deps.Auth == nil {
				return nil, errUnavailable("Authentification indisponible.")
			}
			member, ok := sessionUser(p.Context)
			if !ok {
				if err := checkCSRF(p.Context); err != nil && refreshCookie(p.Context) != "" {
					return nil, err
				}
				_, userID, err := deps.Auth.SessionFromRefresh(p.Context, refreshCookie(p.Context))
				if err != nil {
					return nil, err
				}
				if err := deps.Auth.LogoutAll(p.Context, userID); err != nil {
					return nil, err
				}
				clearSessionCookies(p.Context)
				return true, nil
			}
			if err := deps.Auth.LogoutAll(p.Context, member.UserID); err != nil {
				return nil, err
			}
			clearSessionCookies(p.Context)
			return true, nil
		}},
		"revokeSession": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Auth == nil {
					if err == nil {
						return nil, errUnavailable("Authentification requise.")
					}
					return nil, err
				}
				id, err := uuid.Parse(str(p.Args["id"]))
				if err != nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				if err := deps.Auth.Revoke(p.Context, member.UserID, id); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"changePassword": &graphql.Field{
			Type: graphql.NewNonNull(sessionType),
			Args: graphql.FieldConfigArgument{
				"currentPassword": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"newPassword":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"client":          &graphql.ArgumentConfig{Type: graphql.String},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Auth == nil {
					if err == nil {
						return nil, errUnavailable("Authentification requise.")
					}
					return nil, err
				}
				result, err := deps.Auth.ChangePassword(p.Context, member.UserID, member.SessionID, str(p.Args["currentPassword"]), str(p.Args["newPassword"]), requestFrom(p.Context))
				if err != nil {
					return nil, err
				}
				return sessionMap(p.Context, result, str(p.Args["client"])), nil
			},
		},
		"resetPassword": &graphql.Field{
			Type: graphql.NewNonNull(sessionType),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(resetInput())}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if deps.Auth == nil {
					return nil, errUnavailable("Authentification indisponible.")
				}
				in := inputMap(p.Args["input"])
				result, err := deps.Auth.ResetPassword(p.Context, str(in["grant"]), str(in["phone"]), str(in["password"]), requestFrom(p.Context))
				if err != nil {
					return nil, err
				}
				return sessionMap(p.Context, result, str(in["client"])), nil
			},
		},
		"createEmployee": &graphql.Field{
			Type: graphql.NewNonNull(employeeType),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(employeeInput())}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil || deps.Auth == nil {
					if err == nil {
						return nil, errUnavailable("Authentification requise.")
					}
					return nil, err
				}
				in := inputMap(p.Args["input"])
				storeID, err := argUUID(in["storeId"])
				if err != nil {
					return nil, err
				}
				row, err := deps.Auth.CreateEmployee(p.Context, member, authn.EmployeeInput{
					FullName: str(in["fullName"]), Phone: str(in["phone"]), Role: str(in["role"]),
					Password: str(in["password"]), StoreID: storeID,
				})
				if err != nil {
					return nil, err
				}
				return employeeMap(row), nil
			},
		},
		"updateEmployeeRole": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"userId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"role":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil || deps.Auth == nil {
					if err == nil {
						return nil, errUnavailable("Authentification requise.")
					}
					return nil, err
				}
				id, err := uuid.Parse(str(p.Args["userId"]))
				if err != nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				if err := deps.Auth.UpdateEmployeeRole(p.Context, member, id, str(p.Args["role"])); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"deactivateEmployee": employeeStatus(deps, false),
		"reactivateEmployee": employeeStatus(deps, true),
		"submitSubscriptionPayment": &graphql.Field{
			Type: graphql.NewNonNull(graphql.ID),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(paymentInput())}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil || deps.Billing == nil {
					if err == nil {
						return nil, errUnavailable("Abonnement indisponible.")
					}
					return nil, err
				}
				in := inputMap(p.Args["input"])
				amount, err := decimal.NewFromString(str(in["declaredAmount"]))
				if err != nil {
					return nil, domain.Invalid("amount", "Montant invalide.")
				}
				id, err := deps.Billing.Submit(p.Context, member, billing.PaymentInput{
					ExternalReference: str(in["externalReference"]), DeclaredAmount: amount, ProofKey: str(in["proofKey"]),
				})
				if err != nil {
					return nil, err
				}
				return id.String(), nil
			},
		},
		"approveSubscriptionPayment": &graphql.Field{
			Type: graphql.NewNonNull(graphql.DateTime),
			Args: graphql.FieldConfigArgument{
				"id":             &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"receivedAmount": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Billing == nil {
					if err == nil {
						return nil, errUnavailable("Abonnement indisponible.")
					}
					return nil, err
				}
				id, err := uuid.Parse(str(p.Args["id"]))
				if err != nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				amount, err := decimal.NewFromString(str(p.Args["receivedAmount"]))
				if err != nil {
					return nil, domain.Invalid("amount", "Montant invalide.")
				}
				expires, err := deps.Billing.Approve(p.Context, member, id, amount)
				if err != nil {
					return nil, err
				}
				return expires, nil
			},
		},
		"rejectSubscriptionPayment": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"id":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
				"reason": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Billing == nil {
					if err == nil {
						return nil, errUnavailable("Abonnement indisponible.")
					}
					return nil, err
				}
				id, err := uuid.Parse(str(p.Args["id"]))
				if err != nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				if err := deps.Billing.Reject(p.Context, member, id, str(p.Args["reason"])); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"updateReminderPreferences": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{"smsRemindersEnabled": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireCompany(p.Context)
				if err != nil || deps.Billing == nil {
					if err == nil {
						return nil, errUnavailable("Abonnement indisponible.")
					}
					return nil, err
				}
				enabled, _ := p.Args["smsRemindersEnabled"].(bool)
				if err := deps.Billing.SetReminders(p.Context, member, enabled); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"upsertPaymentInstructions": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{"input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(instructionInput())}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Billing == nil {
					if err == nil {
						return nil, errUnavailable("Abonnement indisponible.")
					}
					return nil, err
				}
				in := inputMap(p.Args["input"])
				if err := deps.Billing.UpsertInstructions(p.Context, member, billing.InstructionInput{
					OperatorName: str(in["operatorName"]), DestinationAccount: str(in["destinationAccount"]),
					BeneficiaryName: str(in["beneficiaryName"]), Currency: str(in["currency"]), Instructions: str(in["instructions"]),
				}); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"updateSubscriptionPlan": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"code":           &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"monthlyPrice":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"durationMonths": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Billing == nil {
					if err == nil {
						return nil, errUnavailable("Abonnement indisponible.")
					}
					return nil, err
				}
				price, err := decimal.NewFromString(str(p.Args["monthlyPrice"]))
				if err != nil {
					return nil, domain.Invalid("plan", "Prix invalide.")
				}
				months := argLimit(p.Args["durationMonths"])
				if err := deps.Billing.UpdatePlan(p.Context, member, str(p.Args["code"]), price, months); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"registerPushDevice": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"token":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"platform": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Auth == nil {
					if err == nil {
						return nil, errUnavailable("Authentification requise.")
					}
					return nil, err
				}
				if err := deps.Auth.RegisterPushDevice(p.Context, member.UserID, str(p.Args["token"]), str(p.Args["platform"])); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"unregisterPushDevice": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{"token": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Auth == nil {
					if err == nil {
						return nil, errUnavailable("Authentification requise.")
					}
					return nil, err
				}
				if err := deps.Auth.UnregisterPushDevice(p.Context, member.UserID, str(p.Args["token"])); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
		"markNotificationRead": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				member, err := requireUser(p.Context)
				if err != nil || deps.Billing == nil {
					if err == nil {
						return nil, errUnavailable("Authentification requise.")
					}
					return nil, err
				}
				id, err := uuid.Parse(str(p.Args["id"]))
				if err != nil {
					return nil, errUnavailable("Identifiant invalide.")
				}
				if err := deps.Billing.MarkRead(p.Context, member.UserID, id); err != nil {
					return nil, err
				}
				return true, nil
			},
		},
	}
	return queries, mutations
}

func requireUser(ctx context.Context) (tenant.Principal, error) {
	member, ok := sessionUser(ctx)
	if !ok {
		return tenant.Principal{}, errUnavailable("Authentification requise.")
	}
	return member, nil
}

func otpRequest(deps Deps, purpose string) *graphql.Field {
	return &graphql.Field{
		Type: graphql.NewNonNull(graphql.Boolean),
		Args: graphql.FieldConfigArgument{"phone": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if deps.Auth == nil {
				return nil, errUnavailable("Authentification indisponible.")
			}
			if err := deps.Auth.RequestOTP(p.Context, purpose, str(p.Args["phone"]), requestFrom(p.Context)); err != nil {
				return nil, err
			}
			return true, nil
		},
	}
}

func otpVerify(deps Deps, purpose string) *graphql.Field {
	return &graphql.Field{
		Type: graphql.NewNonNull(graphql.String),
		Args: graphql.FieldConfigArgument{
			"phone": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"code":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
		},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			if deps.Auth == nil {
				return nil, errUnavailable("Authentification indisponible.")
			}
			return deps.Auth.VerifyOTP(p.Context, purpose, str(p.Args["phone"]), str(p.Args["code"]))
		},
	}
}

func employeeMap(row authn.Employee) map[string]any {
	return map[string]any{
		"userId": row.UserID.String(), "fullName": row.FullName, "phone": row.Phone,
		"role": row.Role, "status": row.Status, "mustChangePassword": row.MustChangePassword,
		"phoneVerified": row.PhoneVerified, "storeIds": idList(row.StoreIDs),
	}
}

func idList(ids []uuid.UUID) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func employeeStatus(deps Deps, reactivate bool) *graphql.Field {
	return &graphql.Field{
		Type: graphql.NewNonNull(graphql.Boolean),
		Args: graphql.FieldConfigArgument{"userId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}},
		Resolve: func(p graphql.ResolveParams) (any, error) {
			member, err := requireCompany(p.Context)
			if err != nil || deps.Auth == nil {
				if err == nil {
					return nil, errUnavailable("Authentification requise.")
				}
				return nil, err
			}
			id, err := uuid.Parse(str(p.Args["userId"]))
			if err != nil {
				return nil, errUnavailable("Identifiant invalide.")
			}
			if reactivate {
				err = deps.Auth.ReactivateEmployee(p.Context, member, id)
			} else {
				err = deps.Auth.DeactivateEmployee(p.Context, member, id)
			}
			if err != nil {
				return nil, err
			}
			return true, nil
		},
	}
}

func logoutOne(p graphql.ResolveParams, deps Deps) (any, error) {
	if deps.Auth == nil {
		return nil, errUnavailable("Authentification indisponible.")
	}
	member, ok := sessionUser(p.Context)
	if !ok {
		if refreshCookie(p.Context) != "" {
			if err := checkCSRF(p.Context); err != nil {
				return nil, err
			}
		}
		sessionID, userID, err := deps.Auth.SessionFromRefresh(p.Context, refreshCookie(p.Context))
		if err != nil {
			return nil, err
		}
		if err := deps.Auth.Logout(p.Context, sessionID, userID); err != nil {
			return nil, err
		}
		clearSessionCookies(p.Context)
		return true, nil
	}
	if err := deps.Auth.Logout(p.Context, member.SessionID, member.UserID); err != nil {
		return nil, err
	}
	clearSessionCookies(p.Context)
	return true, nil
}

func screenMap(screen billing.Screen) map[string]any {
	out := map[string]any{
		"companyId": screen.CompanyID.String(), "companyName": screen.CompanyName, "role": screen.Role,
		"lifecycle": screen.Lifecycle, "status": screen.Status, "planCode": screen.PlanCode, "planName": screen.PlanName,
		"timezone": screen.Timezone, "message": screen.Message, "banner": screen.Banner,
		"canDeclarePayment": screen.CanDeclarePayment,
	}
	if screen.Role == "owner" {
		out["amount"] = screen.Amount
		out["currency"] = screen.Currency
		out["durationMonths"] = screen.DurationMonths
		if screen.Instructions != nil {
			out["renewalReference"] = screen.Instructions.RenewalReference
			out["operatorName"] = screen.Instructions.OperatorName
			out["destinationAccount"] = screen.Instructions.DestinationAccount
			out["beneficiaryName"] = screen.Instructions.BeneficiaryName
			out["paymentInstructions"] = screen.Instructions.Instructions
		}
		if screen.LatestPayment != nil {
			out["paymentStatus"] = screen.LatestPayment.Status
			out["paymentReason"] = screen.LatestPayment.DecisionReason
			if screen.LatestPayment.NewExpiresAt != nil {
				out["newExpiresAt"] = *screen.LatestPayment.NewExpiresAt
			}
		}
	}
	if screen.ExpiresAt != nil {
		out["expiresAt"] = *screen.ExpiresAt
	}
	return out
}

func paymentsMap(rows []billing.PaymentView) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"id": row.ID.String(), "status": row.Status, "expectedAmount": row.ExpectedAmount,
			"declaredAmount": row.DeclaredAmount, "currency": row.Currency,
			"externalReference": row.ExternalReference, "decisionReason": row.DecisionReason,
			"createdAt": row.CreatedAt,
		}
		if row.NewExpiresAt != nil {
			item["newExpiresAt"] = *row.NewExpiresAt
		}
		out = append(out, item)
	}
	return out
}

func sessionMap(ctx context.Context, result authn.SessionResult, client string) map[string]any {
	out := map[string]any{
		"accessToken": result.AccessToken, "expiresAt": result.ExpiresAt, "sessionId": result.SessionID.String(),
		"userId": result.UserID.String(), "role": result.Role, "scope": result.Scope,
		"mustChangePassword": result.MustChangePassword, "csrfToken": result.CSRFToken,
	}
	if result.CompanyID != uuid.Nil {
		out["companyId"] = result.CompanyID.String()
	}
	if client == "web" {
		rememberWebSession(ctx, result)
	} else {
		out["refreshToken"] = result.RefreshToken
	}
	return out
}

func inputMap(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	if mapped == nil {
		return map[string]any{}
	}
	return mapped
}

func signupInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CompleteOwnerSignupInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"grant":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"phone":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"fullName":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"password":        &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"email":           &graphql.InputObjectFieldConfig{Type: graphql.String},
			"companyName":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"primaryCurrency": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"timezone":        &graphql.InputObjectFieldConfig{Type: graphql.String},
			"client":          &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
}

func loginInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "LoginInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"phone":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"password":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"companyId": &graphql.InputObjectFieldConfig{Type: graphql.ID},
			"client":    &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
}

func resetInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "ResetPasswordInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"grant":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"phone":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"password": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"client":   &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
}

func employeeInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "CreateEmployeeInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"fullName": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"phone":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"role":     &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"password": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"storeId":  &graphql.InputObjectFieldConfig{Type: graphql.ID},
		},
	})
}

func paymentInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "SubmitSubscriptionPaymentInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"externalReference": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"declaredAmount":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"proofKey":          &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	})
}

func instructionInput() *graphql.InputObject {
	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "PaymentInstructionsInput",
		Fields: graphql.InputObjectConfigFieldMap{
			"operatorName":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"destinationAccount": &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"beneficiaryName":    &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"currency":           &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
			"instructions":       &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
		},
	})
}

func fromGRPC(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	reason := ""
	for _, detail := range st.Details() {
		if info, ok := detail.(*errdetails.ErrorInfo); ok {
			reason = info.Reason
		}
	}
	if reason == "" {
		return err
	}
	code := domain.CodeInternal
	switch st.Code() {
	case 16:
		code = domain.CodeUnauthenticated
	case 7:
		code = domain.CodePermissionDenied
	case 3:
		code = domain.CodeInvalid
	case 5:
		code = domain.CodeNotFound
	case 6:
		code = domain.CodeAlreadyExists
	case 9:
		code = domain.CodeFailedPrecondition
	case 10:
		code = domain.CodeConflict
	}
	return domain.New(code, reason, st.Message())
}

// requestFrom is filled by the HTTP layer.
func requestFrom(ctx context.Context) authn.Request {
	meta, _ := ctx.Value(httpMetaKey{}).(httpMeta)
	return authn.Request{IP: meta.IP, UserAgent: meta.UserAgent}
}

func refreshCookie(ctx context.Context) string {
	meta, _ := ctx.Value(httpMetaKey{}).(httpMeta)
	return meta.RefreshCookie
}

func checkCSRF(ctx context.Context) error {
	meta, _ := ctx.Value(httpMetaKey{}).(httpMeta)
	if meta.CSRFCookie == "" || meta.CSRFCookie != meta.CSRFHeader {
		return domain.PermissionDenied("csrf", "Jeton CSRF manquant ou invalide.")
	}
	return nil
}
