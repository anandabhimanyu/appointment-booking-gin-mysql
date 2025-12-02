-- Migrations for appointment booking backend

CREATE TABLE IF NOT EXISTS coaches (
  id INT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(255) DEFAULT NULL,
  timezone VARCHAR(64) NOT NULL,
  created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
  id INT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(255) DEFAULT NULL,
  created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS coach_availabilities (
  id INT AUTO_INCREMENT PRIMARY KEY,
  coach_id INT NOT NULL,
  day_of_week TINYINT NOT NULL, -- 0=Sunday..6=Saturday
  start_time VARCHAR(5) NOT NULL, -- '09:00'
  end_time   VARCHAR(5) NOT NULL, -- '14:30'
  created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_coach_day (coach_id, day_of_week),
  CONSTRAINT fk_ca_coach FOREIGN KEY (coach_id) REFERENCES coaches(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS bookings (
  id INT AUTO_INCREMENT PRIMARY KEY,
  user_id INT NOT NULL,
  coach_id INT NOT NULL,
  start_time DATETIME(6) NOT NULL,
  end_time   DATETIME(6) NOT NULL,
  created_at DATETIME(6) DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT fk_b_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  CONSTRAINT fk_b_coach FOREIGN KEY (coach_id) REFERENCES coaches(id) ON DELETE CASCADE,
  UNIQUE KEY ux_coach_start (coach_id, start_time)
);
