# Services Overview

This directory contains the gRPC service definitions (proto files) for the tokenized real estate trading platform. Each service is responsible for a specific domain or functionality within the platform.

## Summary Table

| Service                  | Status      | Purpose/Notes                                 |
|--------------------------|-------------|-----------------------------------------------|
| Wallet/Account Service   | (Optional)  | Manage user balances, deposits, withdrawals, and transfers (add if custodial) |
| Compliance/Document      | (Optional)  | Manage KYC/AML documents and compliance workflows (add if complex) |
| Fee/Commission           | (Optional)  | Manage trading fees, commissions, and rebates (add if complex) |
| Audit/Logging            | (Optional)  | Centralized audit trail for compliance/security (add if needed) |
| Reporting/Analytics      | (Optional)  | Generate reports for users, admins, compliance, or analytics (add if needed) |
| Document/Media Storage   | (Optional)  | Manage property images, legal docs, and files (add if needed) |
| OrderDataService         | Present     | User-centric order updates and history         |
| OrderMonitor             | Present     | Admin/system-centric order monitoring, alerts, and stats |
| PropertyService          | Present     | Asset (property) management only              |
| ListingService           | Present     | Market/trading management for listings/tokens  |
| MarketDataSource         | Present     | Internal aggregation of market data sources    |
| MarketDataService        | Present     | Provides market data to clients               |
| NotificationService      | Present     | User/system notifications                     |
| UserService              | Present     | User management, KYC, roles, preferences      |
| TokenService             | Present     | Token lifecycle and metadata management       |
| TradeService             | Present     | Trade history, streaming, analytics           |
| SettlementService        | Present     | Post-trade asset/cash transfer and settlement |
| ExecutionVenue           | Present     | Venue/market routing and metadata             |
| ClientConfigService      | Present     | Frontend config and feature flags             |
| Login/AuthService        | Present     | Authentication and session management         |

---

## Service Descriptions

- **Wallet/Account Service**: (Optional) Manages user balances, deposits, withdrawals, and internal transfers. Essential for custodial platforms.
- **Compliance/Document Service**: (Optional) Handles KYC/AML documents and compliance workflows.
- **Fee/Commission Service**: (Optional) Manages trading fees, commissions, and rebates.
- **Audit/Logging Service**: (Optional) Centralized audit trail for compliance and security.
- **Reporting/Analytics Service**: (Optional) Generates reports for users, admins, compliance, or analytics.
- **Document/Media Storage Service**: (Optional) Manages property images, legal documents, and other files.
- **OrderDataService**: Provides user-centric order updates and order history.
- **OrderMonitor**: Provides admin/system-centric order monitoring, alerts, and statistics.
- **PropertyService**: Manages real-world property assets and their metadata.
- **ListingService**: Manages tradable instances of properties (tokens) on specific markets/venues.
- **MarketDataSource**: Aggregates market data from various sources (exchanges, oracles, etc.).
- **MarketDataService**: Provides real-time and historical market data to clients.
- **NotificationService**: Manages user and system notifications.
- **UserService**: Handles user account management, KYC, roles, and preferences.
- **TokenService**: Manages the lifecycle and metadata of tokens.
- **TradeService**: Manages trade history, real-time trade streaming, and analytics.
- **SettlementService**: Handles post-trade settlement and asset/cash transfer.
- **ExecutionVenue**: Manages venue/market routing and metadata.
- **ClientConfigService**: Provides frontend configuration and feature flags.
- **Login/AuthService**: Handles authentication and session management.

---

**Note:** Optional services can be added as your platform grows or as compliance, analytics, or business needs increase. Keep PropertyService and ListingService separate to avoid duplication and ensure a clean architecture. 