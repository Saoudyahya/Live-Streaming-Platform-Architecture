from passlib.context import CryptContext
from jose import JWTError, jwt
from datetime import datetime, timedelta
from fastapi import HTTPException, status
from app.config.settings import settings
import secrets
import traceback

# Initialize password context
try:
    pwd_context = CryptContext(schemes=["bcrypt"], deprecated="auto")
    print("✅ Password context initialized successfully")
except Exception as e:
    print(f"❌ Failed to initialize password context: {e}")
    raise


def hash_password(password: str) -> str:
    try:
        if not password:
            raise ValueError("Password cannot be empty")

        print(f"🔐 Hashing password (length: {len(password)})")
        hashed = pwd_context.hash(password)
        print(f"✅ Password hashed successfully (hash length: {len(hashed)})")
        return hashed
    except Exception as e:
        print(f"❌ Error hashing password: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise


def verify_password(plain_password: str, hashed_password: str) -> bool:
    try:
        if not plain_password or not hashed_password:
            print("❌ Empty password or hash provided")
            return False

        print(f"🔍 Verifying password (plain length: {len(plain_password)}, hash length: {len(hashed_password)})")
        result = pwd_context.verify(plain_password, hashed_password)
        print(f"✅ Password verification result: {result}")
        return result
    except Exception as e:
        print(f"❌ Error verifying password: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        return False


def create_access_token(data: dict) -> str:
    try:
        if not data:
            raise ValueError("Token data cannot be empty")

        print(f"🎫 Creating access token for data: {data}")

        to_encode = data.copy()
        expire = datetime.utcnow() + timedelta(minutes=settings.JWT_ACCESS_TOKEN_EXPIRE_MINUTES)
        to_encode.update({"exp": expire, "type": "access"})

        print(f"🔐 Token will expire at: {expire}")
        print(f"🔑 Using JWT secret key (length: {len(settings.JWT_SECRET_KEY)})")

        token = jwt.encode(to_encode, settings.JWT_SECRET_KEY, algorithm=settings.JWT_ALGORITHM)
        print(f"✅ Access token created successfully (length: {len(token)})")

        return token
    except Exception as e:
        print(f"❌ Error creating access token: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise


def create_refresh_token(data: dict) -> str:
    try:
        if not data:
            raise ValueError("Token data cannot be empty")

        print(f"🎫 Creating refresh token for data: {data}")

        to_encode = data.copy()
        expire = datetime.utcnow() + timedelta(days=settings.JWT_REFRESH_TOKEN_EXPIRE_DAYS)
        to_encode.update({"exp": expire, "type": "refresh"})

        print(f"🔐 Refresh token will expire at: {expire}")

        token = jwt.encode(to_encode, settings.JWT_SECRET_KEY, algorithm=settings.JWT_ALGORITHM)
        print(f"✅ Refresh token created successfully (length: {len(token)})")

        return token
    except Exception as e:
        print(f"❌ Error creating refresh token: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise


def verify_token(token: str) -> dict:
    try:
        if not token:
            raise ValueError("Token cannot be empty")

        print(f"🔍 Verifying token (length: {len(token)})")
        print(f"🔑 Using JWT secret key (length: {len(settings.JWT_SECRET_KEY)})")

        payload = jwt.decode(token, settings.JWT_SECRET_KEY, algorithms=[settings.JWT_ALGORITHM])
        print(f"✅ Token verified successfully. Payload: {payload}")

        return payload
    except JWTError as e:
        print(f"❌ JWT Error verifying token: {e}")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=f"Invalid token: {str(e)}",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except Exception as e:
        print(f"❌ Unexpected error verifying token: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=f"Token verification failed: {str(e)}",
            headers={"WWW-Authenticate": "Bearer"},
        )


def generate_stream_key() -> str:
    try:
        print("🔑 Generating stream key...")
        stream_key = secrets.token_urlsafe(32)
        print(f"✅ Stream key generated successfully (length: {len(stream_key)})")
        return stream_key
    except Exception as e:
        print(f"❌ Error generating stream key: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        raise


# Test all security functions on import
def test_security_functions():
    try:
        print("🧪 Testing security functions...")

        # Test password hashing
        test_password = "test123"
        hashed = hash_password(test_password)
        verified = verify_password(test_password, hashed)

        if not verified:
            raise Exception("Password verification failed")

        # Test token creation
        test_data = {"sub": "123", "email": "test@example.com"}
        access_token = create_access_token(test_data)
        refresh_token = create_refresh_token(test_data)

        # Test token verification
        payload = verify_token(access_token)

        if payload.get("sub") != "123":
            raise Exception("Token payload verification failed")

        # Test stream key generation
        stream_key = generate_stream_key()

        if len(stream_key) < 10:
            raise Exception("Stream key too short")

        print("✅ All security function tests passed")
        return True

    except Exception as e:
        print(f"❌ Security function test failed: {e}")
        print(f"🔍 Traceback: {traceback.format_exc()}")
        return False


# Run tests on import
print("🔧 Initializing security module...")
if test_security_functions():
    print("✅ Security module initialized successfully")
else:
    print("❌ Security module initialization failed")