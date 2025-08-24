from pydantic_settings import BaseSettings
from typing import List
import os


class Settings(BaseSettings):
    # Database - Fixed the URL scheme
    DATABASE_URL: str = os.getenv(
        "DATABASE_URL",
        "postgresql://postgres:yahyasd56@localhost:5432/User-service"
    )

    # JWT
    JWT_SECRET_KEY: str = os.getenv("JWT_SECRET_KEY", "your-secret-key-change-in-production")
    JWT_ALGORITHM: str = "HS256"
    JWT_ACCESS_TOKEN_EXPIRE_MINUTES: int = 30
    JWT_REFRESH_TOKEN_EXPIRE_DAYS: int = 7

    # Security
    PASSWORD_MIN_LENGTH: int = 8
    BCRYPT_ROUNDS: int = 12

    # CORS
    ALLOWED_ORIGINS: List[str] = ["http://localhost:3000", "http://localhost:8080"]

    # App
    DEBUG: bool = os.getenv("DEBUG", "false").lower() == "true"
    APP_NAME: str = "User Service"

    # AWS
    AWS_REGION: str = os.getenv("AWS_REGION", "us-east-1")

    class Config:
        case_sensitive = True


settings = Settings()