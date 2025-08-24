from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session
from app.config.database import get_db
from app.schemas.auth import LoginRequest, TokenResponse, RefreshTokenRequest, ChangePasswordRequest
from app.schemas.user import UserCreate, UserResponse
from app.services.auth_service import AuthService
from app.services.user_service import UserService
from app.api.dependencies import get_current_user
from app.models.user import User
import traceback

router = APIRouter()


@router.post("/register", response_model=UserResponse, status_code=status.HTTP_201_CREATED)
async def register(user_data: UserCreate, db: Session = Depends(get_db)):
    try:
        print(f"📝 Register attempt for user: {user_data.username}, email: {user_data.email}")

        user_service = UserService(db)
        result = user_service.create_user(user_data)

        print(f"✅ User created successfully: {result.username} (ID: {result.id})")
        return result

    except HTTPException as he:
        print(f"❌ HTTP Exception in register: {he.detail}")
        raise he
    except Exception as e:
        print(f"💥 Unexpected error in register: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Registration failed: {str(e)}"
        )


@router.post("/login", response_model=TokenResponse)
async def login(login_data: LoginRequest, db: Session = Depends(get_db)):
    try:
        print(f"🔐 Login attempt for email: {login_data.email}")

        # Test database connection
        from sqlalchemy import text
        db.execute(text("SELECT 1"))
        print("✅ Database connection test passed in login")

        auth_service = AuthService(db)
        result = auth_service.authenticate_user(login_data)

        print(f"✅ Login successful for: {login_data.email}")
        return result

    except HTTPException as he:
        print(f"❌ HTTP Exception in login: {he.detail}")
        raise he
    except Exception as e:
        print(f"💥 Unexpected error in login: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Login failed: {str(e)}"
        )


@router.post("/refresh", response_model=TokenResponse)
async def refresh_token(
        refresh_data: RefreshTokenRequest,
        db: Session = Depends(get_db)
):
    try:
        print(f"🔄 Token refresh attempt")

        auth_service = AuthService(db)
        result = auth_service.refresh_access_token(refresh_data.refresh_token)

        print(f"✅ Token refresh successful")
        return result

    except HTTPException as he:
        print(f"❌ HTTP Exception in refresh: {he.detail}")
        raise he
    except Exception as e:
        print(f"💥 Unexpected error in refresh: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Token refresh failed: {str(e)}"
        )


@router.post("/logout")
async def logout(
        refresh_data: RefreshTokenRequest,
        db: Session = Depends(get_db)
):
    try:
        print(f"👋 Logout attempt")

        auth_service = AuthService(db)
        success = auth_service.logout(refresh_data.refresh_token)

        message = "Logged out successfully" if success else "Already logged out"
        print(f"✅ Logout result: {message}")
        return {"message": message}

    except Exception as e:
        print(f"💥 Unexpected error in logout: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Logout failed: {str(e)}"
        )


@router.post("/change-password")
async def change_password(
        password_data: ChangePasswordRequest,
        current_user: User = Depends(get_current_user),
        db: Session = Depends(get_db)
):
    try:
        print(f"🔒 Password change attempt for user: {current_user.username}")

        user_service = UserService(db)
        success = user_service.change_password(
            current_user.id,
            password_data.current_password,
            password_data.new_password
        )

        print(f"✅ Password change successful for user: {current_user.username}")
        return {"message": "Password changed successfully"}

    except HTTPException as he:
        print(f"❌ HTTP Exception in change_password: {he.detail}")
        raise he
    except Exception as e:
        print(f"💥 Unexpected error in change_password: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Password change failed: {str(e)}"
        )


@router.get("/me", response_model=UserResponse)
async def get_current_user_info(current_user: User = Depends(get_current_user)):
    try:
        print(f"👤 Get current user info for: {current_user.username}")
        return UserResponse.model_validate(current_user)
    except Exception as e:
        print(f"💥 Unexpected error in get_current_user_info: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Failed to get user info: {str(e)}"
        )


# Debug endpoint to test basic functionality
@router.get("/debug/test")
async def debug_test(db: Session = Depends(get_db)):
    try:
        print("🧪 Running debug test...")

        # Test database
        from sqlalchemy import text
        result = db.execute(text("SELECT 1 as test"))
        row = result.fetchone()

        # Test model imports
        from app.models.user import User
        from app.models.auth import RefreshToken

        # Test services
        from app.services.auth_service import AuthService
        from app.services.user_service import UserService

        # Test security utils
        from app.utils.security import hash_password, generate_stream_key

        test_password = hash_password("testpass123")
        test_stream_key = generate_stream_key()

        print("✅ All debug tests passed")

        return {
            "status": "success",
            "database": "connected" if row and row[0] == 1 else "error",
            "models": "imported",
            "services": "imported",
            "security": "working",
            "test_password_hash": len(test_password) > 0,
            "test_stream_key": len(test_stream_key) > 0
        }

    except Exception as e:
        print(f"💥 Debug test failed: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        return {
            "status": "error",
            "error": str(e),
            "traceback": traceback.format_exc()
        }