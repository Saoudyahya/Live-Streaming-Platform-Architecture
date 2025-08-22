# services/user-service/app/grpc_server/user_service.py
import grpc
import socket
from concurrent import futures
from sqlalchemy.orm import Session
from datetime import datetime

from app.proto.user import user_service_pb2_grpc, user_service_pb2
from app.proto.common import common_pb2, timestamp_pb2
from app.config.database import SessionLocal
from app.repository.user_repository import UserRepository
from app.models.user import User


class UserServicer(user_service_pb2_grpc.UserServiceServicer):
    def __init__(self):
        pass

    def _get_db(self) -> Session:
        """Get database session"""
        return SessionLocal()

    def _create_status(self, success: bool, code: int = 0, message: str = "") -> common_pb2.Status:
        """Create a gRPC Status message"""
        return common_pb2.Status(
            code=code,
            message=message,
            success=success
        )

    def _datetime_to_timestamp(self, dt: datetime) -> timestamp_pb2.Timestamp:
        """Convert Python datetime to gRPC Timestamp"""
        if not dt:
            return None
        timestamp = timestamp_pb2.Timestamp()
        timestamp.seconds = int(dt.timestamp())
        timestamp.nanos = int((dt.timestamp() % 1) * 1e9)
        return timestamp

    def _user_to_proto(self, user: User) -> user_service_pb2.User:
        """Convert User model to gRPC User message"""
        return user_service_pb2.User(
            id=str(user.id),
            username=user.username or "",
            email=user.email or "",
            display_name=f"{user.first_name or ''} {user.last_name or ''}".strip(),
            avatar_url=user.profile_image_url or "",
            status=user_service_pb2.UserStatus.ONLINE if user.is_active else user_service_pb2.UserStatus.OFFLINE,
            created_at=self._datetime_to_timestamp(user.created_at),
            last_seen=self._datetime_to_timestamp(user.updated_at or user.created_at)
        )

    def GetUser(self, request, context):
        """Get a single user by ID"""
        db = self._get_db()
        try:
            user_repo = UserRepository(db)

            # Convert string ID to int
            try:
                user_id = int(request.user_id)
            except ValueError:
                return user_service_pb2.GetUserResponse(
                    status=self._create_status(False, 400, "Invalid user ID format")
                )

            user = user_repo.get_user_by_id(user_id)

            if not user:
                return user_service_pb2.GetUserResponse(
                    status=self._create_status(False, 404, "User not found")
                )

            return user_service_pb2.GetUserResponse(
                status=self._create_status(True, 200, "User retrieved successfully"),
                user=self._user_to_proto(user)
            )

        except Exception as e:
            print(f"Error in GetUser: {e}")
            return user_service_pb2.GetUserResponse(
                status=self._create_status(False, 500, f"Internal server error: {str(e)}")
            )
        finally:
            db.close()

    def GetUsers(self, request, context):
        """Get multiple users by IDs"""
        db = self._get_db()
        try:
            user_repo = UserRepository(db)
            users = []

            for user_id_str in request.user_ids:
                try:
                    user_id = int(user_id_str)
                    user = user_repo.get_user_by_id(user_id)
                    if user:
                        users.append(self._user_to_proto(user))
                except ValueError:
                    continue  # Skip invalid IDs

            return user_service_pb2.GetUsersResponse(
                status=self._create_status(True, 200, "Users retrieved successfully"),
                users=users
            )

        except Exception as e:
            print(f"Error in GetUsers: {e}")
            return user_service_pb2.GetUsersResponse(
                status=self._create_status(False, 500, f"Internal server error: {str(e)}")
            )
        finally:
            db.close()

    def ValidateUser(self, request, context):
        """Validate user credentials - simplified for chat service"""
        db = self._get_db()
        try:
            user_repo = UserRepository(db)

            # For now, just check if user exists and is active
            # In a real implementation, you'd validate the token
            try:
                user_id = int(request.user_id)
            except ValueError:
                return user_service_pb2.ValidateUserResponse(
                    status=self._create_status(False, 400, "Invalid user ID format"),
                    is_valid=False
                )

            user = user_repo.get_user_by_id(user_id)

            if not user or not user.is_active:
                return user_service_pb2.ValidateUserResponse(
                    status=self._create_status(False, 401, "Invalid or inactive user"),
                    is_valid=False
                )

            return user_service_pb2.ValidateUserResponse(
                status=self._create_status(True, 200, "User validated successfully"),
                is_valid=True,
                user=self._user_to_proto(user)
            )

        except Exception as e:
            print(f"Error in ValidateUser: {e}")
            return user_service_pb2.ValidateUserResponse(
                status=self._create_status(False, 500, f"Internal server error: {str(e)}"),
                is_valid=False
            )
        finally:
            db.close()

    def UpdateUserStatus(self, request, context):
        """Update user status"""
        db = self._get_db()
        try:
            user_repo = UserRepository(db)

            try:
                user_id = int(request.user_id)
            except ValueError:
                return user_service_pb2.UpdateUserStatusResponse(
                    status=self._create_status(False, 400, "Invalid user ID format")
                )

            user = user_repo.get_user_by_id(user_id)

            if not user:
                return user_service_pb2.UpdateUserStatusResponse(
                    status=self._create_status(False, 404, "User not found")
                )

            # Update user status based on the gRPC status
            if request.status == user_service_pb2.UserStatus.OFFLINE:
                user.is_active = False
            else:
                user.is_active = True

            db.commit()

            return user_service_pb2.UpdateUserStatusResponse(
                status=self._create_status(True, 200, "User status updated successfully")
            )

        except Exception as e:
            print(f"Error in UpdateUserStatus: {e}")
            return user_service_pb2.UpdateUserStatusResponse(
                status=self._create_status(False, 500, f"Internal server error: {str(e)}")
            )
        finally:
            db.close()


def find_free_port(start_port: int = 8082, max_port: int = 8092) -> int:
    """Find a free port starting from start_port"""
    for port in range(start_port, max_port):
        try:
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                s.bind(('localhost', port))
                return port
        except OSError:
            continue
    raise RuntimeError(f"No free port found in range {start_port}-{max_port}")


def serve_grpc(port: int = None) -> grpc.Server:
    """Start the gRPC server"""
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    user_service_pb2_grpc.add_UserServiceServicer_to_server(UserServicer(), server)

    # Find a free port if the specified port is not available
    if port is None:
        port = find_free_port()

    # Try different binding approaches
    bind_addresses = [
        f'localhost:{port}',  # Try localhost first
        f'127.0.0.1:{port}',  # Try IPv4 explicitly
        f'0.0.0.0:{port}',  # Try all interfaces
    ]

    bound = False
    actual_port = port

    for addr in bind_addresses:
        try:
            actual_port = server.add_insecure_port(addr)
            print(f"🚀 gRPC server bound to {addr} (port {actual_port})")
            bound = True
            break
        except RuntimeError as e:
            print(f"❌ Failed to bind to {addr}: {e}")
            continue

    # If all specific addresses fail, try to find any free port
    if not bound:
        try:
            free_port = find_free_port(8082, 8100)
            actual_port = server.add_insecure_port(f'localhost:{free_port}')
            print(f"🚀 gRPC server bound to localhost:{free_port} (port {actual_port})")
            bound = True
        except Exception as e:
            print(f"❌ Failed to find any free port: {e}")
            raise RuntimeError("Failed to start gRPC server - no available ports")

    if not bound:
        raise RuntimeError("Failed to bind gRPC server to any address")

    server.start()
    print(f"✅ gRPC server started and listening on port {actual_port}")

    return server