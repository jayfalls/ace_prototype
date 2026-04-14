# Entity Relationship Diagram

```mermaid
erDiagram
    users ||--o{ resource_permissions : "user_id"
    users ||--o{ sessions : "user_id"
```
