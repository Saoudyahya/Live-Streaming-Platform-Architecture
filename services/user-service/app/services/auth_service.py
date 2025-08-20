from sqlalchemy.orm import Session
from datetime import datetime, timedelta
from typing import Optional
from app.repository.user_repository import UserRepository
from app.models.auth import RefreshToken
from app.schemas.auth import LoginRequest, TokenResponse
from app.utils.security import verify_password, create_access_token, create_refresh_token, verify_token
from app.utils.exceptions import InvalidCredentialsError, InactiveUserError
from app.config.settings import settings
import secrets


class AuthService:
    def __init__(self, db: Session):
        self.db = db
        self.user_repo = UserRepository(db)

    def authenticate_user(self, login_data: LoginRequest) -> TokenResponse:
        user = self.user_repo.get_user_by_email(login_data.email)

        if not user or not verify_password(login_data.password, user.hashed_password):
            raise InvalidCredentialsError()

        if not user.is_active:
            raise InactiveUserError()

        # Create tokens
        access_token = create_access_token({"sub": str(user.id), "email": user.email})
        refresh_token = create_refresh_token({"sub": str(user.id)})

        # Store refresh token
        self._store_refresh_token(user.id, refresh_token)

        return TokenResponse(
            access_token=access_token,
            refresh_token=refresh_token,
            expires_in=settings.JWT_ACCESS_TOKEN_EXPIRE_MINUTES * 60
        )

    def refresh_access_token(self, refresh_token: str) -> TokenResponse:
        # Verify refresh token
        payload = verify_token(refresh_token)

        if payload.get("type") != "refresh":
            raise InvalidCredentialsError()

        user_id = int(payload.get("sub"))

        # Check if refresh token exists in database
        db_token = self.db.query(RefreshToken).filter(
            RefreshToken.token == refresh_token,
            RefreshToken.user_id == user_id
        ).first()

        if not db_token:
            raise InvalidCredentialsError()

        # Check if token is expired
        if db_token.expires_at < datetime.utcnow():
            self.db.delete(db_token)
            self.db.commit()
            raise InvalidCredentialsError()

        # Get user
        user = self.user_repo.get_user_by_id(user_id)
        if not user or not user.is_active:
            raise InvalidCredentialsError()

        # Create new tokens
        new_access_token = create_access_token({"sub": str(user.id), "email": user.email})
        new_refresh_token = create_refresh_token({"sub": str(user.id)})

        # Update refresh token in database
        db_token.token = new_refresh_token
        db_token.expires_at = datetime.utcnow() + timedelta(days=settings.JWT_REFRESH_TOKEN_EXPIRE_DAYS)
        self.db.commit()

        return TokenResponse(
            access_token=new_access_token,
            refresh_token=new_refresh_token,
            expires_in=settings.JWT_ACCESS_TOKEN_EXPIRE_MINUTES * 60
        )

    def logout(self, refresh_token: str) -> bool:
        db_token = self.db.query(RefreshToken).filter(
            RefreshToken.token == refresh_token
        ).first()

        if db_token:
            self.db.delete(db_token)
            self.db.commit()
            return True

        return False

    def _store_refresh_token(self, user_id: int, token: str):
        # Remove existing refresh tokens for user
        self.db.query(RefreshToken).filter(RefreshToken.user_id == user_id).delete()

        # Create new refresh token
        db_token = RefreshToken(
            token=token,
            user_id=user_id,
            expires_at=datetime.utcnow() + timedelta(days=settings.JWT_REFRESH_TOKEN_EXPIRE_DAYS)
        )

        self.db.add(db_token)
        self.db.commit()