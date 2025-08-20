from sqlalchemy.orm import Session
from typing import List, Optional
from app.repository.user_repository import UserRepository
from app.schemas.user import UserCreate, UserUpdate, UserResponse
from app.models.user import User
from app.utils.exceptions import UserAlreadyExistsError, UserNotFoundError
from app.utils.security import verify_password


class UserService:
    def __init__(self, db: Session):
        self.db = db
        self.user_repo = UserRepository(db)

    def create_user(self, user_data: UserCreate) -> UserResponse:
        # Check if user already exists
        existing_user = self.user_repo.get_user_by_email(user_data.email)
        if existing_user:
            raise UserAlreadyExistsError()

        existing_username = self.user_repo.get_user_by_username(user_data.username)
        if existing_username:
            raise UserAlreadyExistsError()

        # Create user
        user = self.user_repo.create_user(user_data)
        return UserResponse.from_orm(user)

    def get_user_by_id(self, user_id: int) -> UserResponse:
        user = self.user_repo.get_user_by_id(user_id)
        if not user:
            raise UserNotFoundError()
        return UserResponse.from_orm(user)

    def get_users(self, skip: int = 0, limit: int = 100) -> List[UserResponse]:
        users = self.user_repo.get_users(skip=skip, limit=limit)
        return [UserResponse.from_orm(user) for user in users]

    def update_user(self, user_id: int, user_data: UserUpdate) -> UserResponse:
        user = self.user_repo.update_user(user_id, user_data)
        if not user:
            raise UserNotFoundError()
        return UserResponse.from_orm(user)

    def delete_user(self, user_id: int) -> bool:
        success = self.user_repo.delete_user(user_id)
        if not success:
            raise UserNotFoundError()
        return True

    def change_password(self, user_id: int, current_password: str, new_password: str) -> bool:
        user = self.user_repo.get_user_by_id(user_id)
        if not user:
            raise UserNotFoundError()

        if not verify_password(current_password, user.hashed_password):
            raise InvalidCredentialsError()

        return self.user_repo.update_password(user_id, new_password)

