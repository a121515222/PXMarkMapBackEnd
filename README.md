
指令說明
go run main.go sync              # 手動同步資料
go run main.go serve             # 啟動 API (http://localhost:8080)
go run main.go schedule          # 啟動排程器
go run main.go serve-schedule    # API + 排程一起跑

資料庫建立

psql -U postgres -c "CREATE DATABASE px_mark_map_db;"

-- 切換到新資料庫
\c px_mark_map_db

-- 建立 stores 表
CREATE TABLE stores (
    id SERIAL PRIMARY KEY,
    store_name VARCHAR(255) NOT NULL UNIQUE,
    place_id VARCHAR(255),
    formatted_address TEXT,
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 建立 shipments 表
CREATE TABLE shipments (
    id SERIAL PRIMARY KEY,
    store_id INTEGER REFERENCES stores(id) ON DELETE CASCADE,
    product_type VARCHAR(50) NOT NULL,
    shipment_date DATE NOT NULL,
    quantity VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(store_id, product_type, shipment_date)
);

-- 建立索引
CREATE INDEX idx_stores_store_name ON stores(store_name);
CREATE INDEX idx_shipments_store_id ON shipments(store_id);
CREATE INDEX idx_shipments_date ON shipments(shipment_date);
CREATE INDEX idx_shipments_product_type ON shipments(product_type);

-- 確認表格建立成功
\dt

CREATE TABLE sync_logs (
    id SERIAL PRIMARY KEY,
    start_time TIMESTAMP NOT NULL,      -- 開始時間
    end_time TIMESTAMP,                  -- 結束時間
    status VARCHAR(20) NOT NULL,         -- 狀態: running/success/failed
    message TEXT,                        -- 訊息
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);