Read

``` mermaid
sequenceDiagram
    participant app
    participant database
    
    app->>database: Get value
    database-->>app: Value
```

Write

``` mermaid
sequenceDiagram
    participant app
    participant database
    
    app->>database: Set value
```