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
import traceback


class AuthService:
    def __init__(self, db: Session):
        print("🔧 Initializing AuthService...")
        self.db = db
        self.user_repo = UserRepository(db)
        print("✅ AuthService initialized successfully")

    def authenticate_user(self, login_data: LoginRequest) -> TokenResponse:
        try:
            print(f"🔐 Authenticating user: {login_data.email}")

            # Find user by email
            print(f"🔍 Looking up user by email: {login_data.email}")
            user = self.user_repo.get_user_by_email(login_data.email)

            if not user:
                print(f"❌ User not found: {login_data.email}")
                raise InvalidCredentialsError()

            print(f"✅ User found: {user.username} (ID: {user.id})")

            # Verify password
            print(f"🔒 Verifying password for user: {user.username}")
            password_valid = verify_password(login_data.password, user.hashed_password)

            if not password_valid:
                print(f"❌ Invalid password for user: {user.username}")
                raise InvalidCredentialsError()

            print(f"✅ Password valid for user: {user.username}")

            # Check if user is active
            if not user.is_active:
                print(f"❌ User is inactive: {user.username}")
                raise InactiveUserError()

            print(f"✅ User is active: {user.username}")

            # Create tokens
            print(f"🎫 Creating tokens for user: {user.username}")
            token_data = {"sub": str(user.id), "email": user.email}

            access_token = create_access_token(token_data)
            refresh_token = create_refresh_token({"sub": str(user.id)})

            print(f"✅ Tokens created for user: {user.username}")

            # Store refresh token
            print(f"💾 Storing refresh token for user: {user.username}")
            self._store_refresh_token(user.id, refresh_token)

            print(f"✅ Authentication successful for user: {user.username}")

            return TokenResponse(
                access_token=access_token,
                refresh_token=refresh_token,
                expires_in=settings.JWT_ACCESS_TOKEN_EXPIRE_MINUTES * 60
            )

        except (InvalidCredentialsError, InactiveUserError) as e:
            print(f"❌ Authentication failed: {e.detail}")
            raise e
        except Exception as e:
            print(f"💥 Unexpected error in authenticate_user: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"Authentication failed: {str(e)}")

    def refresh_access_token(self, refresh_token: str) -> TokenResponse:
        try:
            print(f"🔄 Refreshing access token...")

            # Verify refresh token
            print(f"🔍 Verifying refresh token...")
            payload = verify_token(refresh_token)

            if payload.get("type") != "refresh":
                print(f"❌ Token is not a refresh token")
                raise InvalidCredentialsError()

            user_id = int(payload.get("sub"))
            print(f"✅ Refresh token valid for user ID: {user_id}")

            # Check if refresh token exists in database
            print(f"🔍 Looking up refresh token in database...")
            db_token = self.db.query(RefreshToken).filter(
                RefreshToken.token == refresh_token,
                RefreshToken.user_id == user_id
            ).first()

            if not db_token:
                print(f"❌ Refresh token not found in database")
                raise InvalidCredentialsError()

            print(f"✅ Refresh token found in database")

            # Check if token is expired
            if db_token.expires_at < datetime.utcnow():
                print(f"❌ Refresh token expired")
                self.db.delete(db_token)
                self.db.commit()
                raise InvalidCredentialsError()

            print(f"✅ Refresh token not expired")

            # Get user
            print(f"🔍 Getting user for ID: {user_id}")
            user = self.user_repo.get_user_by_id(user_id)
            if not user or not user.is_active:
                print(f"❌ User not found or inactive")
                raise InvalidCredentialsError()

            print(f"✅ User found and active: {user.username}")

            # Create new tokens
            print(f"🎫 Creating new tokens...")
            new_access_token = create_access_token({"sub": str(user.id), "email": user.email})
            new_refresh_token = create_refresh_token({"sub": str(user.id)})

            # Update refresh token in database
            print(f"💾 Updating refresh token in database...")
            db_token.token = new_refresh_token
            db_token.expires_at = datetime.utcnow() + timedelta(days=settings.JWT_REFRESH_TOKEN_EXPIRE_DAYS)
            self.db.commit()

            print(f"✅ Token refresh successful for user: {user.username}")

            return TokenResponse(
                access_token=new_access_token,
                refresh_token=new_refresh_token,
                expires_in=settings.JWT_ACCESS_TOKEN_EXPIRE_MINUTES * 60
            )

        except (InvalidCredentialsError, InactiveUserError) as e:
            print(f"❌ Token refresh failed: {e.detail}")
            raise e
        except Exception as e:
            print(f"💥 Unexpected error in refresh_access_token: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"Token refresh failed: {str(e)}")

    def logout(self, refresh_token: str) -> bool:
        try:
            print(f"👋 Logging out user...")

            db_token = self.db.query(RefreshToken).filter(
                RefreshToken.token == refresh_token
            ).first()

            if db_token:
                print(f"✅ Refresh token found, deleting...")
                self.db.delete(db_token)
                self.db.commit()
                print(f"✅ Logout successful")
                return True
            else:
                print(f"ℹ️ Refresh token not found (already logged out)")
                return False

        except Exception as e:
            print(f"💥 Unexpected error in logout: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            return False

    def _store_refresh_token(self, user_id: int, token: str):
        try:
            print(f"💾 Storing refresh token for user ID: {user_id}")

            # Remove existing refresh tokens for user
            deleted_count = self.db.query(RefreshToken).filter(RefreshToken.user_id == user_id).delete()
            print(f"🗑️ Deleted {deleted_count} existing refresh tokens")

            # Create new refresh token
            expires_at = datetime.utcnow() + timedelta(days=settings.JWT_REFRESH_TOKEN_EXPIRE_DAYS)
            print(f"📅 Refresh token will expire at: {expires_at}")

            db_token = RefreshToken(
                token=token,
                user_id=user_id,
                expires_at=expires_at
            )

            self.db.add(db_token)
            self.db.commit()

            print(f"✅ Refresh token stored successfully for user ID: {user_id}")

        except Exception as e:
            print(f"💥 Error storing refresh token: {e}")
            print(f"🔍 Traceback: {traceback.format_exc()}")
            raise Exception(f"Failed to store refresh token: {str(e)}")